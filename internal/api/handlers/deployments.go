package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/sshexec"
	"github.com/gsarma/localisprod-v2/internal/store"
)

type DeploymentHandler struct {
	store *store.Store
}

func NewDeploymentHandler(s *store.Store) *DeploymentHandler {
	return &DeploymentHandler{store: s}
}

func (h *DeploymentHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	var body struct {
		ApplicationID string `json:"application_id"`
		NodeID        string `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ApplicationID == "" || body.NodeID == "" {
		writeError(w, http.StatusBadRequest, "application_id and node_id are required")
		return
	}

	app, err := h.store.GetApplication(body.ApplicationID, userID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	node, err := h.store.GetNodeForUser(body.NodeID, userID, isRoot(r))
	if err != nil || node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if node.IsLocal && !isRoot(r) {
		writeError(w, http.StatusForbidden, "only the root user can deploy to the management node")
		return
	}

	// Check for port conflicts on each host port declared by the application.
	var appPorts []string
	_ = json.Unmarshal([]byte(app.Ports), &appPorts)
	runner := sshexec.NewRunner(node)
	for _, mapping := range appPorts {
		// mapping format: "hostPort:containerPort"
		hostPort := mapping
		if idx := strings.LastIndex(mapping, ":"); idx >= 0 {
			hostPort = mapping[:idx]
		}
		var port int
		if _, scanErr := fmt.Sscanf(hostPort, "%d", &port); scanErr != nil || port == 0 {
			continue
		}
		if port < 1 || port > 65535 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid port %d: must be between 1 and 65535", port))
			return
		}
		if used, err := h.store.IsPortUsedOnNode(body.NodeID, port); err != nil {
			writeInternalError(w, err)
			return
		} else if used {
			writeError(w, http.StatusConflict, fmt.Sprintf("port %d is already in use on this node", port))
			return
		}
		if used, _ := sshexec.IsPortInUse(runner, port); used {
			writeError(w, http.StatusConflict, fmt.Sprintf("port %d is already bound on the node", port))
			return
		}
	}

	shortID := uuid.New().String()[:8]
	containerName := fmt.Sprintf("localisprod-%s-%s", strings.ReplaceAll(app.Name, " ", "-"), shortID)

	deployment := &models.Deployment{
		ID:            uuid.New().String(),
		ApplicationID: body.ApplicationID,
		NodeID:        body.NodeID,
		ContainerName: containerName,
		ContainerID:   "",
		Status:        "pending",
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.store.CreateDeployment(deployment, userID); err != nil {
		writeInternalError(w, err)
		return
	}

	// Parse env vars and ports
	var envVars map[string]string
	_ = json.Unmarshal([]byte(app.EnvVars), &envVars)
	var ports []string
	_ = json.Unmarshal([]byte(app.Ports), &ports)

	// Inject database connection URLs from linked databases
	var dbIDs []string
	_ = json.Unmarshal([]byte(app.Databases), &dbIDs)
	for _, dbID := range dbIDs {
		db, err := h.store.GetDatabase(dbID, userID)
		if err != nil || db == nil {
			continue
		}
		if envVars == nil {
			envVars = map[string]string{}
		}
		envVars[DBEnvVarName(db.Name)] = DBConnectionURL(db)
	}
	// For single-database apps, also inject DATABASE_URL as a convenience alias
	// unless the user has already set it explicitly.
	if len(dbIDs) == 1 {
		if _, exists := envVars["DATABASE_URL"]; !exists {
			db, err := h.store.GetDatabase(dbIDs[0], userID)
			if err == nil && db != nil {
				envVars["DATABASE_URL"] = DBConnectionURL(db)
			}
		}
	}

	// Inject cache connection URLs from linked caches
	var cacheIDs []string
	_ = json.Unmarshal([]byte(app.Caches), &cacheIDs)
	for _, cID := range cacheIDs {
		c, err := h.store.GetCache(cID, userID)
		if err != nil || c == nil {
			continue
		}
		if envVars == nil {
			envVars = map[string]string{}
		}
		envVars[CacheEnvVarName(c.Name)] = CacheConnectionURL(c)
	}
	// For single-cache apps, also inject CACHE_URL as a convenience alias.
	if len(cacheIDs) == 1 {
		if _, exists := envVars["CACHE_URL"]; !exists {
			c, err := h.store.GetCache(cacheIDs[0], userID)
			if err == nil && c != nil {
				envVars["CACHE_URL"] = CacheConnectionURL(c)
			}
		}
	}

	// Inject Kafka bootstrap server addresses from linked Kafka clusters
	var kafkaIDs []string
	_ = json.Unmarshal([]byte(app.Kafkas), &kafkaIDs)
	for _, kID := range kafkaIDs {
		k, err := h.store.GetKafka(kID, userID)
		if err != nil || k == nil {
			continue
		}
		if envVars == nil {
			envVars = map[string]string{}
		}
		envVars[KafkaEnvVarName(k.Name)] = KafkaConnectionURL(k)
	}
	// For single-Kafka apps, also inject KAFKA_BROKERS as a convenience alias.
	if len(kafkaIDs) == 1 {
		if _, exists := envVars["KAFKA_BROKERS"]; !exists {
			k, err := h.store.GetKafka(kafkaIDs[0], userID)
			if err == nil && k != nil {
				envVars["KAFKA_BROKERS"] = KafkaConnectionURL(k)
			}
		}
	}

	// Inject monitoring URLs from linked monitoring stacks
	var monitoringIDs []string
	_ = json.Unmarshal([]byte(app.Monitorings), &monitoringIDs)
	for _, mID := range monitoringIDs {
		mon, err := h.store.GetMonitoring(mID, userID)
		if err != nil || mon == nil {
			continue
		}
		if envVars == nil {
			envVars = map[string]string{}
		}
		envVars[MonitoringPrometheusEnvVarName(mon.Name)] = MonitoringPrometheusURL(mon)
		envVars[MonitoringGrafanaEnvVarName(mon.Name)] = MonitoringGrafanaURL(mon)
	}
	// For single-monitoring apps, also inject convenience aliases.
	if len(monitoringIDs) == 1 {
		mon, err := h.store.GetMonitoring(monitoringIDs[0], userID)
		if err == nil && mon != nil {
			if _, exists := envVars["PROMETHEUS_URL"]; !exists {
				envVars["PROMETHEUS_URL"] = MonitoringPrometheusURL(mon)
			}
			if _, exists := envVars["GRAFANA_URL"]; !exists {
				envVars["GRAFANA_URL"] = MonitoringGrafanaURL(mon)
			}
		}
	}

	// If image is from ghcr.io, authenticate first
	if strings.HasPrefix(app.DockerImage, "ghcr.io/") {
		ghToken, _ := h.store.GetUserSetting(userID, "github_token")
		ghUsername, _ := h.store.GetUserSetting(userID, "github_username")
		if ghToken != "" && ghUsername != "" {
			loginCmd := sshexec.DockerLoginCmd(ghUsername, ghToken)
			loginOutput, loginErr := runner.Run(loginCmd)
			if loginErr != nil {
				_ = h.store.UpdateDeploymentStatus(deployment.ID, userID, "failed", "")
				deployment.Status = "failed"
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"deployment": deployment,
					"error":      "docker login failed: " + loginErr.Error(),
					"output":     loginOutput,
				})
				return
			}
		}
	}

	// Write env vars to a temporary file on the node so they are never
	// exposed in the process list or shell history.
	var envFilePath string
	if len(envVars) > 0 {
		envFilePath = fmt.Sprintf("/tmp/%s.env", containerName)
		var buf strings.Builder
		for k, v := range envVars {
			buf.WriteString(k)
			buf.WriteByte('=')
			buf.WriteString(v)
			buf.WriteByte('\n')
		}
		if err := runner.WriteFile(envFilePath, buf.String()); err != nil {
			_ = h.store.UpdateDeploymentStatus(deployment.ID, userID, "failed", "")
			deployment.Status = "failed"
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"deployment": deployment,
				"error":      "failed to write env file: " + err.Error(),
			})
			return
		}
	}

	cfg := sshexec.RunConfig{
		ContainerName: containerName,
		Image:         app.DockerImage,
		Ports:         ports,
		EnvFilePath:   envFilePath,
		CommandArgs:   shellFields(app.Command),
	}

	if app.Domain != "" {
		containerPort := "80"
		if len(ports) > 0 {
			// Parse "hostPort:containerPort" → take container side
			if idx := strings.LastIndex(ports[0], ":"); idx >= 0 {
				containerPort = ports[0][idx+1:]
			}
		}
		cfg.Network = "traefik-net"
		cfg.Labels = sshexec.TraefikLabels(containerName, app.Domain, containerPort)
	}

	cmd := sshexec.DockerRunCmd(cfg)
	output, runErr := runner.Run(cmd)

	// Always remove the env file — docker run -d has already loaded it.
	if envFilePath != "" {
		_, _ = runner.Run(sshexec.RemoveFileCmd(envFilePath))
	}

	if runErr != nil {
		_ = h.store.UpdateDeploymentStatus(deployment.ID, userID, "failed", "")
		deployment.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"deployment": deployment,
			"error":      runErr.Error(),
			"output":     output,
		})
		return
	}

	containerID := strings.TrimSpace(output)
	now := time.Now().UTC()
	_ = h.store.UpdateDeploymentStatus(deployment.ID, userID, "running", containerID)
	_ = h.store.UpdateDeploymentLastDeployedAt(deployment.ID, userID, now)
	_ = h.store.UpdateApplicationLastDeployedAt(body.ApplicationID, userID, now)
	deployment.Status = "running"
	deployment.ContainerID = containerID
	deployment.LastDeployedAt = &now

	writeJSON(w, http.StatusCreated, deployment)
}

func (h *DeploymentHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	deployments, err := h.store.ListDeployments(userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if deployments == nil {
		deployments = []*models.Deployment{}
	}
	writeJSON(w, http.StatusOK, deployments)
}

func (h *DeploymentHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	d, err := h.store.GetDeployment(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if d == nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *DeploymentHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	d, err := h.store.GetDeployment(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if d == nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	node, err := h.store.GetNodeForUser(d.NodeID, userID, isRoot(r))
	if err == nil && node != nil {
		cmd := sshexec.DockerStopRemoveCmd(d.ContainerName)
		_, _ = sshexec.NewRunner(node).Run(cmd)
	}

	if err := h.store.DeleteDeployment(id, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeploymentHandler) Restart(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	d, err := h.store.GetDeployment(id, userID)
	if err != nil || d == nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	node, err := h.store.GetNodeForUser(d.NodeID, userID, isRoot(r))
	if err != nil || node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	cmd := sshexec.DockerRestartCmd(d.ContainerName)
	output, runErr := sshexec.NewRunner(node).Run(cmd)

	if runErr != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "error",
			"message": runErr.Error(),
			"output":  output,
		})
		return
	}

	now := time.Now().UTC()
	_ = h.store.UpdateDeploymentStatus(id, userID, "running", d.ContainerID)
	_ = h.store.UpdateDeploymentLastDeployedAt(id, userID, now)
	_ = h.store.UpdateApplicationLastDeployedAt(d.ApplicationID, userID, now)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "running",
		"message": "container restarted",
	})
}

func (h *DeploymentHandler) Logs(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	d, err := h.store.GetDeployment(id, userID)
	if err != nil || d == nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	node, err := h.store.GetNodeForUser(d.NodeID, userID, isRoot(r))
	if err != nil || node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	cmd := sshexec.DockerLogsCmd(d.ContainerName)
	output, runErr := sshexec.NewRunner(node).Run(cmd)

	if runErr != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"logs":  output,
			"error": runErr.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"logs": output,
	})
}
