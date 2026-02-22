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

	app, err := h.store.GetApplication(body.ApplicationID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "application not found")
		return
	}

	node, err := h.store.GetNode(body.NodeID)
	if err != nil || node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
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
	if err := h.store.CreateDeployment(deployment); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Parse env vars and ports
	var envVars map[string]string
	_ = json.Unmarshal([]byte(app.EnvVars), &envVars)
	var ports []string
	_ = json.Unmarshal([]byte(app.Ports), &ports)

	runner := sshexec.NewRunner(node)

	// If image is from ghcr.io, authenticate first
	if strings.HasPrefix(app.DockerImage, "ghcr.io/") {
		ghToken, _ := h.store.GetSetting("github_token")
		ghUsername, _ := h.store.GetSetting("github_username")
		if ghToken != "" && ghUsername != "" {
			loginCmd := sshexec.DockerLoginCmd(ghUsername, ghToken)
			loginOutput, loginErr := runner.Run(loginCmd)
			if loginErr != nil {
				_ = h.store.UpdateDeploymentStatus(deployment.ID, "failed", "")
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

	cmd := sshexec.DockerRunCmd(containerName, app.DockerImage, ports, envVars, app.Command)
	output, runErr := runner.Run(cmd)

	if runErr != nil {
		_ = h.store.UpdateDeploymentStatus(deployment.ID, "failed", "")
		deployment.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"deployment": deployment,
			"error":      runErr.Error(),
			"output":     output,
		})
		return
	}

	containerID := strings.TrimSpace(output)
	_ = h.store.UpdateDeploymentStatus(deployment.ID, "running", containerID)
	deployment.Status = "running"
	deployment.ContainerID = containerID

	writeJSON(w, http.StatusCreated, deployment)
}

func (h *DeploymentHandler) List(w http.ResponseWriter, r *http.Request) {
	deployments, err := h.store.ListDeployments()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if deployments == nil {
		deployments = []*models.Deployment{}
	}
	writeJSON(w, http.StatusOK, deployments)
}

func (h *DeploymentHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	d, err := h.store.GetDeployment(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if d == nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *DeploymentHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	d, err := h.store.GetDeployment(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if d == nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	node, err := h.store.GetNode(d.NodeID)
	if err == nil && node != nil {
		cmd := sshexec.DockerStopRemoveCmd(d.ContainerName)
		_, _ = sshexec.NewRunner(node).Run(cmd)
	}

	if err := h.store.DeleteDeployment(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeploymentHandler) Restart(w http.ResponseWriter, r *http.Request, id string) {
	d, err := h.store.GetDeployment(id)
	if err != nil || d == nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	node, err := h.store.GetNode(d.NodeID)
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

	_ = h.store.UpdateDeploymentStatus(id, "running", d.ContainerID)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "running",
		"message": "container restarted",
	})
}

func (h *DeploymentHandler) Logs(w http.ResponseWriter, r *http.Request, id string) {
	d, err := h.store.GetDeployment(id)
	if err != nil || d == nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	node, err := h.store.GetNode(d.NodeID)
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
