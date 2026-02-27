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

type CacheHandler struct {
	store *store.Store
}

func NewCacheHandler(s *store.Store) *CacheHandler {
	return &CacheHandler{store: s}
}

func (h *CacheHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	var body struct {
		Name     string   `json:"name"`
		Version  string   `json:"version"`
		NodeID   string   `json:"node_id"`
		Password string   `json:"password"`
		Port     int      `json:"port"`
		Volumes  []string `json:"volumes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.NodeID == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "name, node_id, and password are required")
		return
	}

	version := body.Version
	if version == "" {
		version = "7"
	}
	port := body.Port
	if port == 0 {
		port = 6379
	}

	node, err := h.store.GetNodeForUser(body.NodeID, userID, isRoot(r))
	if err != nil || node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if node.IsLocal && !isRoot(r) {
		writeError(w, http.StatusForbidden, "only the root user can create caches on the management node")
		return
	}

	// Check for port conflicts before creating the container.
	if used, err := h.store.IsPortUsedOnNode(body.NodeID, port); err != nil {
		writeInternalError(w, err)
		return
	} else if used {
		writeError(w, http.StatusConflict, fmt.Sprintf("port %d is already in use on this node", port))
		return
	}
	runner := sshexec.NewRunner(node)
	if used, _ := sshexec.IsPortInUse(runner, port); used {
		writeError(w, http.StatusConflict, fmt.Sprintf("port %d is already bound on the node", port))
		return
	}

	safeName := strings.ReplaceAll(body.Name, " ", "-")
	shortID := uuid.New().String()[:8]
	containerName := fmt.Sprintf("localisprod-db-%s-%s", safeName, shortID)

	// Resolve volumes: user-supplied or default named volume
	var volumes []string
	if len(body.Volumes) > 0 {
		volumes = body.Volumes
	} else {
		volumeName := fmt.Sprintf("localisprod-%s-data", safeName)
		_, _ = runner.Run(sshexec.DockerVolumeCreateCmd(volumeName))
		volumes = []string{fmt.Sprintf("%s:/data", volumeName)}
	}

	volumesJSON, _ := json.Marshal(volumes)

	c := &models.Cache{
		ID:            uuid.New().String(),
		Name:          body.Name,
		Version:       version,
		NodeID:        body.NodeID,
		Password:      body.Password,
		Port:          port,
		Volumes:       string(volumesJSON),
		ContainerName: containerName,
		Status:        "pending",
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.store.CreateCache(c, userID); err != nil {
		writeInternalError(w, err)
		return
	}

	runCfg := sshexec.RunConfig{
		ContainerName: containerName,
		Image:         fmt.Sprintf("redis:%s", version),
		Ports:         []string{fmt.Sprintf("%d:6379", port)},
		Volumes:       volumes,
		Restart:       "unless-stopped",
		Command:       fmt.Sprintf("redis-server --requirepass %s", sshexec.ShellEscape(body.Password)),
	}

	cmd := sshexec.DockerRunCmd(runCfg)
	output, runErr := runner.Run(cmd)

	if runErr != nil {
		_ = h.store.UpdateCacheStatus(c.ID, userID, "failed")
		c.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"cache":  c,
			"error":  runErr.Error(),
			"output": output,
		})
		return
	}

	now := time.Now().UTC()
	_ = h.store.UpdateCacheStatus(c.ID, userID, "running")
	_ = h.store.UpdateCacheLastDeployedAt(c.ID, userID, now)
	c.Status = "running"
	c.LastDeployedAt = &now
	writeJSON(w, http.StatusCreated, c)
}

func (h *CacheHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	caches, err := h.store.ListCaches(userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if caches == nil {
		caches = []*models.Cache{}
	}
	writeJSON(w, http.StatusOK, caches)
}

func (h *CacheHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	c, err := h.store.GetCache(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if c == nil {
		writeError(w, http.StatusNotFound, "cache not found")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *CacheHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	c, err := h.store.GetCache(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if c == nil {
		writeError(w, http.StatusNotFound, "cache not found")
		return
	}

	node, err := h.store.GetNodeForUser(c.NodeID, userID, isRoot(r))
	if err == nil && node != nil {
		_, _ = sshexec.NewRunner(node).Run(sshexec.DockerStopRemoveCmd(c.ContainerName))
	}

	if err := h.store.DeleteCache(id, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CacheEnvVarName derives the env var name from a cache name.
// e.g. "my-cache" → "MY_CACHE_URL"
func CacheEnvVarName(name string) string {
	return DBEnvVarName(name)
}

// CacheConnectionURL builds the Redis connection URL.
func CacheConnectionURL(c *models.Cache) string {
	return fmt.Sprintf("redis://:%s@%s:%d", c.Password, c.NodeHost, c.Port)
}
