package handlers

import (
	"crypto/rand"
	"encoding/hex"
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

type ObjectStorageHandler struct {
	store *store.Store
}

func NewObjectStorageHandler(s *store.Store) *ObjectStorageHandler {
	return &ObjectStorageHandler{store: s}
}

func (h *ObjectStorageHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}

	var body struct {
		Name    string `json:"name"`
		NodeID  string `json:"node_id"`
		S3Port  int    `json:"s3_port"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.NodeID == "" {
		writeError(w, http.StatusBadRequest, "name and node_id are required")
		return
	}

	version := body.Version
	if version == "" {
		version = "v1.0.1"
	}
	s3Port := body.S3Port
	if s3Port == 0 {
		s3Port = 3900
	}

	node, err := h.store.GetNodeForUser(body.NodeID, userID, isRoot(r))
	if err != nil || node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if node.IsLocal && !isRoot(r) {
		writeError(w, http.StatusForbidden, "only the root user can create object storages on the management node")
		return
	}

	if used, err := h.store.IsPortUsedOnNode(body.NodeID, s3Port); err != nil {
		writeInternalError(w, err)
		return
	} else if used {
		writeError(w, http.StatusConflict, fmt.Sprintf("port %d is already in use on this node", s3Port))
		return
	}
	runner := sshexec.NewRunner(node)
	if used, _ := sshexec.IsPortInUse(runner, s3Port); used {
		writeError(w, http.StatusConflict, fmt.Sprintf("port %d is already bound on the node", s3Port))
		return
	}

	rpcSecretBytes := make([]byte, 32)
	if _, err := rand.Read(rpcSecretBytes); err != nil {
		writeInternalError(w, fmt.Errorf("generate rpc secret: %w", err))
		return
	}
	rpcSecret := hex.EncodeToString(rpcSecretBytes)

	safeName := strings.ReplaceAll(body.Name, " ", "-")
	shortID := uuid.New().String()[:8]
	containerName := fmt.Sprintf("localisprod-garage-%s-%s", safeName, shortID)

	o := &models.ObjectStorage{
		ID:            uuid.New().String(),
		Name:          body.Name,
		Version:       version,
		NodeID:        body.NodeID,
		S3Port:        s3Port,
		ContainerName: containerName,
		Status:        "pending",
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.store.CreateObjectStorage(o, userID, rpcSecret); err != nil {
		writeInternalError(w, err)
		return
	}

	garageConfig := fmt.Sprintf(`metadata_dir = "/var/lib/garage/meta"
data_dir = "/var/lib/garage/data"
db_engine = "sqlite"
replication_factor = 1

rpc_bind_addr = "[::]:3901"
rpc_public_addr = "%s:3901"
rpc_secret = "%s"

[s3_api]
s3_region = "garage"
api_bind_addr = "[::]:3900"

[admin]
api_bind_addr = "[::]:3903"
`, node.Host, rpcSecret)

	configPath := fmt.Sprintf("/tmp/%s.toml", containerName)
	if err := runner.WriteFile(configPath, garageConfig); err != nil {
		_ = h.store.UpdateObjectStorageStatus(o.ID, userID, "failed")
		o.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"object_storage": o,
			"error":          err.Error(),
		})
		return
	}

	runCfg := sshexec.RunConfig{
		ContainerName: containerName,
		Image:         fmt.Sprintf("dxflrs/garage:%s", version),
		Ports:         []string{fmt.Sprintf("%d:3900", s3Port)},
		Volumes: []string{
			fmt.Sprintf("/tmp/%s-meta:/var/lib/garage/meta", safeName),
			fmt.Sprintf("/tmp/%s-data:/var/lib/garage/data", safeName),
			fmt.Sprintf("%s:/etc/garage.toml", configPath),
		},
		Restart: "unless-stopped",
	}
	cmd := sshexec.DockerRunCmd(runCfg)
	output, runErr := runner.Run(cmd)
	if runErr != nil {
		_ = h.store.UpdateObjectStorageStatus(o.ID, userID, "failed")
		o.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"object_storage": o,
			"error":          runErr.Error(),
			"output":         output,
		})
		return
	}

	// Allow Garage time to initialise before running admin commands.
	time.Sleep(2 * time.Second)

	// Configure cluster layout.
	nodeIDOut, nodeIDErr := runner.Run(fmt.Sprintf("docker exec %s /garage node id", sshexec.ShellEscape(containerName)))
	if nodeIDErr == nil {
		// The output of `garage node id` ends with a line of the form
		// "<hex-id>@<host>:<port>". Scan from the end to find it.
		var nodeHexID string
		for _, line := range strings.Split(nodeIDOut, "\n") {
			line = strings.TrimSpace(line)
			if idx := strings.Index(line, "@"); idx > 0 {
				nodeHexID = line[:idx]
			}
		}
		if nodeHexID != "" {
			_, _ = runner.Run(fmt.Sprintf("docker exec %s /garage layout assign %s --zone dc1 --capacity 1073741824",
				sshexec.ShellEscape(containerName), sshexec.ShellEscape(nodeHexID)))
			_, _ = runner.Run(fmt.Sprintf("docker exec %s /garage layout apply --version 1",
				sshexec.ShellEscape(containerName)))
		}
	}

	// Create default access key.
	keyOut, keyErr := runner.Run(fmt.Sprintf("docker exec %s /garage key create default", sshexec.ShellEscape(containerName)))
	var accessKeyID, secretAccessKey string
	if keyErr == nil {
		for _, line := range strings.Split(keyOut, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Key ID:") {
				accessKeyID = strings.TrimSpace(strings.TrimPrefix(line, "Key ID:"))
			} else if strings.HasPrefix(line, "Secret key:") {
				secretAccessKey = strings.TrimSpace(strings.TrimPrefix(line, "Secret key:"))
			}
		}
	}

	now := time.Now().UTC()
	_ = h.store.UpdateObjectStorageStatus(o.ID, userID, "running")
	_ = h.store.UpdateObjectStorageLastDeployedAt(o.ID, userID, now)
	if accessKeyID != "" {
		_ = h.store.UpdateObjectStorageCredentials(o.ID, userID, accessKeyID, secretAccessKey)
	}

	o.Status = "running"
	o.LastDeployedAt = &now
	o.AccessKeyID = accessKeyID
	o.SecretAccessKey = secretAccessKey
	o.NodeHost = node.Host
	o.NodeName = node.Name
	writeJSON(w, http.StatusCreated, o)
}

func (h *ObjectStorageHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	result, err := h.store.ListObjectStorages(userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if result == nil {
		result = []*models.ObjectStorage{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *ObjectStorageHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	o, err := h.store.GetObjectStorage(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if o == nil {
		writeError(w, http.StatusNotFound, "object storage not found")
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (h *ObjectStorageHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	o, err := h.store.GetObjectStorage(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if o == nil {
		writeError(w, http.StatusNotFound, "object storage not found")
		return
	}

	node, err := h.store.GetNodeForUser(o.NodeID, userID, isRoot(r))
	if err == nil && node != nil {
		_, _ = sshexec.NewRunner(node).Run(sshexec.DockerStopRemoveCmd(o.ContainerName))
	}

	if err := h.store.DeleteObjectStorage(id, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ObjectStorageEndpoint returns the S3-compatible endpoint URL.
func ObjectStorageEndpoint(o *models.ObjectStorage) string {
	return fmt.Sprintf("http://%s:%d", o.NodeHost, o.S3Port)
}
