package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gsarma/localisprod-v2/internal/sshexec"
	"github.com/gsarma/localisprod-v2/internal/store"
)

type WebhookHandler struct {
	store *store.Store
}

func NewWebhookHandler(s *store.Store) *WebhookHandler {
	return &WebhookHandler{store: s}
}

func (h *WebhookHandler) Github(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	// Verify HMAC-SHA256 signature if webhook_secret is configured
	secret, _ := h.store.GetSetting("webhook_secret")
	if secret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !verifySignature(secret, sig, body) {
			writeError(w, http.StatusUnauthorized, "invalid signature")
			return
		}
	}

	// Only process registry_package events; acknowledge everything else
	event := r.Header.Get("X-GitHub-Event")
	if event != "registry_package" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "event": event})
		return
	}

	var payload struct {
		Action     string `json:"action"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	repoFullName := payload.Repository.FullName
	if repoFullName == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"redeployed": 0})
		return
	}

	apps, err := h.store.ListApplications()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ghToken, _ := h.store.GetSetting("github_token")
	ghUsername, _ := h.store.GetSetting("github_username")

	redeployed := 0

	for _, app := range apps {
		if app.GithubRepo != repoFullName {
			continue
		}

		deployments, err := h.store.GetDeploymentsByApplicationID(app.ID)
		if err != nil {
			log.Printf("webhook: list deployments for app %s: %v", app.ID, err)
			continue
		}

		var envVars map[string]string
		_ = json.Unmarshal([]byte(app.EnvVars), &envVars)
		var ports []string
		_ = json.Unmarshal([]byte(app.Ports), &ports)

		for _, d := range deployments {
			if d.Status != "running" {
				continue
			}

			node, err := h.store.GetNode(d.NodeID)
			if err != nil || node == nil {
				log.Printf("webhook: node %s not found for deployment %s", d.NodeID, d.ID)
				continue
			}

			runner := sshexec.NewRunner(node)

			// Authenticate with GHCR if needed
			if strings.HasPrefix(app.DockerImage, "ghcr.io/") && ghToken != "" && ghUsername != "" {
				loginCmd := sshexec.DockerLoginCmd(ghUsername, ghToken)
				if _, loginErr := runner.Run(loginCmd); loginErr != nil {
					log.Printf("webhook: docker login failed for deployment %s: %v", d.ID, loginErr)
					_ = h.store.UpdateDeploymentStatus(d.ID, "failed", "")
					continue
				}
			}

			// Pull new image
			pullCmd := sshexec.DockerPullCmd(app.DockerImage)
			if _, pullErr := runner.Run(pullCmd); pullErr != nil {
				log.Printf("webhook: docker pull failed for deployment %s: %v", d.ID, pullErr)
				_ = h.store.UpdateDeploymentStatus(d.ID, "failed", "")
				continue
			}

			// Stop and remove old container
			stopCmd := sshexec.DockerStopRemoveCmd(d.ContainerName)
			if _, stopErr := runner.Run(stopCmd); stopErr != nil {
				log.Printf("webhook: docker stop/rm failed for deployment %s: %v", d.ID, stopErr)
			}

			// Build RunConfig reusing the same container name
			cfg := sshexec.RunConfig{
				ContainerName: d.ContainerName,
				Image:         app.DockerImage,
				Ports:         ports,
				EnvVars:       envVars,
				Command:       app.Command,
			}
			if app.Domain != "" {
				containerPort := "80"
				if len(ports) > 0 {
					if idx := strings.LastIndex(ports[0], ":"); idx >= 0 {
						containerPort = ports[0][idx+1:]
					}
				}
				cfg.Network = "traefik-net"
				cfg.Labels = sshexec.TraefikLabels(d.ContainerName, app.Domain, containerPort)
			}

			runCmd := sshexec.DockerRunCmd(cfg)
			output, runErr := runner.Run(runCmd)
			if runErr != nil {
				log.Printf("webhook: docker run failed for deployment %s: %v\noutput: %s", d.ID, runErr, output)
				_ = h.store.UpdateDeploymentStatus(d.ID, "failed", "")
				continue
			}

			newContainerID := strings.TrimSpace(output)
			_ = h.store.UpdateDeploymentStatus(d.ID, "running", newContainerID)
			log.Printf("webhook: redeployed %s (container %s) on node %s", d.ContainerName, newContainerID, node.Name)
			redeployed++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"redeployed": redeployed,
		"repo":       repoFullName,
	})
}

func verifySignature(secret, sigHeader string, body []byte) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(sigHeader, prefix) {
		return false
	}
	sig, err := hex.DecodeString(sigHeader[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(sig, expected)
}
