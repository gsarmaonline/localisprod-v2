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

type MonitoringHandler struct {
	store *store.Store
}

func NewMonitoringHandler(s *store.Store) *MonitoringHandler {
	return &MonitoringHandler{store: s}
}

func (h *MonitoringHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	var body struct {
		Name            string `json:"name"`
		NodeID          string `json:"node_id"`
		PrometheusPort  int    `json:"prometheus_port"`
		GrafanaPort     int    `json:"grafana_port"`
		GrafanaPassword string `json:"grafana_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.NodeID == "" {
		writeError(w, http.StatusBadRequest, "name and node_id are required")
		return
	}

	if body.PrometheusPort == 0 {
		body.PrometheusPort = 9090
	}
	if body.GrafanaPort == 0 {
		body.GrafanaPort = 3000
	}
	if body.GrafanaPassword == "" {
		body.GrafanaPassword = "admin"
	}

	node, err := h.store.GetNodeForUser(body.NodeID, userID, isRoot(r))
	if err != nil || node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if node.IsLocal && !isRoot(r) {
		writeError(w, http.StatusForbidden, "only the root user can create monitoring stacks on the management node")
		return
	}

	// Check for port conflicts before creating containers.
	runner := sshexec.NewRunner(node)
	for _, port := range []int{body.PrometheusPort, body.GrafanaPort} {
		if used, err := h.store.IsPortUsedOnNode(body.NodeID, port); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
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

	safeName := strings.ReplaceAll(body.Name, " ", "-")
	shortID := uuid.New().String()[:8]
	promContainer := fmt.Sprintf("localisprod-prometheus-%s-%s", safeName, shortID)
	grafanaContainer := fmt.Sprintf("localisprod-grafana-%s-%s", safeName, shortID)
	networkName := fmt.Sprintf("localisprod-monitoring-%s-%s-net", safeName, shortID)
	promVolume := fmt.Sprintf("localisprod-prom-%s-%s-data", safeName, shortID)
	grafanaVolume := fmt.Sprintf("localisprod-grafana-%s-%s-data", safeName, shortID)
	baseDir := fmt.Sprintf("/opt/localisprod/monitoring/%s-%s", safeName, shortID)

	m := &models.Monitoring{
		ID:                      uuid.New().String(),
		Name:                    body.Name,
		NodeID:                  body.NodeID,
		PrometheusPort:          body.PrometheusPort,
		GrafanaPort:             body.GrafanaPort,
		GrafanaPassword:         body.GrafanaPassword,
		PrometheusContainerName: promContainer,
		GrafanaContainerName:    grafanaContainer,
		Status:                  "pending",
		CreatedAt:               time.Now().UTC(),
	}
	if err := h.store.CreateMonitoring(m, userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Create config directory on the node
	mkdirCmd := fmt.Sprintf("mkdir -p %s/grafana-provisioning/datasources", sshexec.ShellEscape(baseDir))
	if _, err := runner.Run(mkdirCmd); err != nil {
		_ = h.store.UpdateMonitoringStatus(m.ID, userID, "failed")
		m.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"monitoring": m,
			"error":      "failed to create config directory: " + err.Error(),
		})
		return
	}

	// Write prometheus.yml
	prometheusConfig := "global:\n  scrape_interval: 15s\n  evaluation_interval: 15s\n\nscrape_configs:\n  - job_name: 'prometheus'\n    static_configs:\n      - targets: ['localhost:9090']\n"
	promConfigPath := baseDir + "/prometheus.yml"
	if err := runner.WriteFile(promConfigPath, prometheusConfig); err != nil {
		_ = h.store.UpdateMonitoringStatus(m.ID, userID, "failed")
		m.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"monitoring": m,
			"error":      "failed to write prometheus config: " + err.Error(),
		})
		return
	}

	// Write Grafana datasource provisioning YAML
	grafanaDS := fmt.Sprintf("apiVersion: 1\ndatasources:\n  - name: Prometheus\n    type: prometheus\n    url: http://%s:9090\n    isDefault: true\n    access: proxy\n", promContainer)
	grafanaDSPath := baseDir + "/grafana-provisioning/datasources/prometheus.yaml"
	if err := runner.WriteFile(grafanaDSPath, grafanaDS); err != nil {
		_ = h.store.UpdateMonitoringStatus(m.ID, userID, "failed")
		m.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"monitoring": m,
			"error":      "failed to write grafana datasource config: " + err.Error(),
		})
		return
	}

	// Write Grafana password env file (temporary — removed after docker run)
	grafanaEnvPath := fmt.Sprintf("/tmp/%s.env", grafanaContainer)
	if err := runner.WriteFile(grafanaEnvPath, fmt.Sprintf("GF_SECURITY_ADMIN_PASSWORD=%s\n", body.GrafanaPassword)); err != nil {
		_ = h.store.UpdateMonitoringStatus(m.ID, userID, "failed")
		m.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"monitoring": m,
			"error":      "failed to write grafana env file: " + err.Error(),
		})
		return
	}

	// Create Docker network (idempotent)
	_, _ = runner.Run(fmt.Sprintf("docker network create %s 2>/dev/null || true", sshexec.ShellEscape(networkName)))

	// Create named volumes
	_, _ = runner.Run(sshexec.DockerVolumeCreateCmd(promVolume))
	_, _ = runner.Run(sshexec.DockerVolumeCreateCmd(grafanaVolume))

	// Run Prometheus container
	promCfg := sshexec.RunConfig{
		ContainerName: promContainer,
		Image:         "prom/prometheus:latest",
		Ports:         []string{fmt.Sprintf("%d:9090", body.PrometheusPort)},
		Volumes: []string{
			fmt.Sprintf("%s:/prometheus", promVolume),
			fmt.Sprintf("%s/prometheus.yml:/etc/prometheus/prometheus.yml:ro", baseDir),
		},
		Network: networkName,
		Restart: "unless-stopped",
	}
	promOutput, promErr := runner.Run(sshexec.DockerRunCmd(promCfg))
	if promErr != nil {
		_, _ = runner.Run(sshexec.RemoveFileCmd(grafanaEnvPath))
		_ = h.store.UpdateMonitoringStatus(m.ID, userID, "failed")
		m.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"monitoring": m,
			"error":      promErr.Error(),
			"output":     promOutput,
		})
		return
	}

	// Run Grafana container
	grafanaCfg := sshexec.RunConfig{
		ContainerName: grafanaContainer,
		Image:         "grafana/grafana:latest",
		Ports:         []string{fmt.Sprintf("%d:3000", body.GrafanaPort)},
		Volumes: []string{
			fmt.Sprintf("%s:/var/lib/grafana", grafanaVolume),
			fmt.Sprintf("%s/grafana-provisioning:/etc/grafana/provisioning:ro", baseDir),
		},
		Network:     networkName,
		EnvFilePath: grafanaEnvPath,
		Restart:     "unless-stopped",
	}
	grafanaOutput, grafanaErr := runner.Run(sshexec.DockerRunCmd(grafanaCfg))

	// Remove temporary Grafana env file
	_, _ = runner.Run(sshexec.RemoveFileCmd(grafanaEnvPath))

	if grafanaErr != nil {
		_ = h.store.UpdateMonitoringStatus(m.ID, userID, "failed")
		m.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"monitoring": m,
			"error":      grafanaErr.Error(),
			"output":     grafanaOutput,
		})
		return
	}

	now := time.Now().UTC()
	_ = h.store.UpdateMonitoringStatus(m.ID, userID, "running")
	_ = h.store.UpdateMonitoringLastDeployedAt(m.ID, userID, now)
	m.Status = "running"
	m.LastDeployedAt = &now
	writeJSON(w, http.StatusCreated, m)
}

func (h *MonitoringHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	monitorings, err := h.store.ListMonitorings(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if monitorings == nil {
		monitorings = []*models.Monitoring{}
	}
	writeJSON(w, http.StatusOK, monitorings)
}

func (h *MonitoringHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	m, err := h.store.GetMonitoring(id, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m == nil {
		writeError(w, http.StatusNotFound, "monitoring stack not found")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *MonitoringHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	m, err := h.store.GetMonitoring(id, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m == nil {
		writeError(w, http.StatusNotFound, "monitoring stack not found")
		return
	}

	node, err := h.store.GetNodeForUser(m.NodeID, userID, isRoot(r))
	if err == nil && node != nil {
		runner := sshexec.NewRunner(node)
		_, _ = runner.Run(sshexec.DockerStopRemoveCmd(m.GrafanaContainerName))
		_, _ = runner.Run(sshexec.DockerStopRemoveCmd(m.PrometheusContainerName))
		// Remove persistent config directory
		baseDirSuffix := strings.TrimPrefix(m.PrometheusContainerName, "localisprod-prometheus-")
		baseDir := "/opt/localisprod/monitoring/" + baseDirSuffix
		_, _ = runner.Run(fmt.Sprintf("rm -rf %s", sshexec.ShellEscape(baseDir)))
	}

	if err := h.store.DeleteMonitoring(id, userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MonitoringPrometheusEnvVarName derives the env var name for the Prometheus URL.
// e.g. "my-monitor" → "MY_MONITOR_PROMETHEUS_URL"
func MonitoringPrometheusEnvVarName(name string) string {
	upper := strings.ToUpper(name)
	cleaned := strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(upper)
	return cleaned + "_PROMETHEUS_URL"
}

// MonitoringGrafanaEnvVarName derives the env var name for the Grafana URL.
// e.g. "my-monitor" → "MY_MONITOR_GRAFANA_URL"
func MonitoringGrafanaEnvVarName(name string) string {
	upper := strings.ToUpper(name)
	cleaned := strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(upper)
	return cleaned + "_GRAFANA_URL"
}

// MonitoringPrometheusURL returns the HTTP URL for the Prometheus HTTP API.
func MonitoringPrometheusURL(m *models.Monitoring) string {
	return fmt.Sprintf("http://%s:%d", m.NodeHost, m.PrometheusPort)
}

// MonitoringGrafanaURL returns the HTTP URL for the Grafana UI.
func MonitoringGrafanaURL(m *models.Monitoring) string {
	return fmt.Sprintf("http://%s:%d", m.NodeHost, m.GrafanaPort)
}
