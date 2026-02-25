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

type KafkaHandler struct {
	store *store.Store
}

func NewKafkaHandler(s *store.Store) *KafkaHandler {
	return &KafkaHandler{store: s}
}

func (h *KafkaHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	var body struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		NodeID  string `json:"node_id"`
		Port    int    `json:"port"`
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
		version = "latest"
	}
	port := body.Port
	if port == 0 {
		port = 9092
	}

	node, err := h.store.GetNodeForUser(body.NodeID, userID, isRoot(r))
	if err != nil || node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if node.IsLocal && !isRoot(r) {
		writeError(w, http.StatusForbidden, "only the root user can create Kafka clusters on the management node")
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
	containerName := fmt.Sprintf("localisprod-kafka-%s-%s", safeName, shortID)
	volumeName := fmt.Sprintf("localisprod-kafka-%s-data", safeName)

	k := &models.Kafka{
		ID:            uuid.New().String(),
		Name:          body.Name,
		Version:       version,
		NodeID:        body.NodeID,
		Port:          port,
		ContainerName: containerName,
		Status:        "pending",
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.store.CreateKafka(k, userID); err != nil {
		writeInternalError(w, err)
		return
	}

	// Create named volume for Kafka data (idempotent)
	_, _ = runner.Run(sshexec.DockerVolumeCreateCmd(volumeName))

	// Write Kafka configuration env vars to a temp file on the node.
	// Using apache/kafka in KRaft mode (no ZooKeeper).
	envFilePath := fmt.Sprintf("/tmp/%s.env", containerName)
	kafkaEnv := fmt.Sprintf(
		"KAFKA_NODE_ID=1\n"+
			"KAFKA_PROCESS_ROLES=broker,controller\n"+
			"KAFKA_CONTROLLER_QUORUM_VOTERS=1@localhost:9093\n"+
			"KAFKA_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093\n"+
			"KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://%s:%d\n"+
			"KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER\n"+
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT\n"+
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1\n"+
			"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1\n"+
			"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1\n",
		node.Host, port,
	)
	if err := runner.WriteFile(envFilePath, kafkaEnv); err != nil {
		_ = h.store.UpdateKafkaStatus(k.ID, userID, "failed")
		k.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"kafka":  k,
			"error":  "failed to write env file: " + err.Error(),
		})
		return
	}

	runCfg := sshexec.RunConfig{
		ContainerName: containerName,
		Image:         fmt.Sprintf("apache/kafka:%s", version),
		Ports:         []string{fmt.Sprintf("%d:9092", port)},
		Volumes:       []string{fmt.Sprintf("%s:/var/lib/kafka/data", volumeName)},
		EnvFilePath:   envFilePath,
		Restart:       "unless-stopped",
	}

	cmd := sshexec.DockerRunCmd(runCfg)
	output, runErr := runner.Run(cmd)

	// Remove the env file once docker run -d has loaded it
	_, _ = runner.Run(sshexec.RemoveFileCmd(envFilePath))

	if runErr != nil {
		_ = h.store.UpdateKafkaStatus(k.ID, userID, "failed")
		k.Status = "failed"
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"kafka":  k,
			"error":  runErr.Error(),
			"output": output,
		})
		return
	}

	now := time.Now().UTC()
	_ = h.store.UpdateKafkaStatus(k.ID, userID, "running")
	_ = h.store.UpdateKafkaLastDeployedAt(k.ID, userID, now)
	k.Status = "running"
	k.LastDeployedAt = &now
	writeJSON(w, http.StatusCreated, k)
}

func (h *KafkaHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	kafkas, err := h.store.ListKafkas(userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if kafkas == nil {
		kafkas = []*models.Kafka{}
	}
	writeJSON(w, http.StatusOK, kafkas)
}

func (h *KafkaHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	k, err := h.store.GetKafka(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if k == nil {
		writeError(w, http.StatusNotFound, "kafka cluster not found")
		return
	}
	writeJSON(w, http.StatusOK, k)
}

func (h *KafkaHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	k, err := h.store.GetKafka(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if k == nil {
		writeError(w, http.StatusNotFound, "kafka cluster not found")
		return
	}

	node, err := h.store.GetNodeForUser(k.NodeID, userID, isRoot(r))
	if err == nil && node != nil {
		_, _ = sshexec.NewRunner(node).Run(sshexec.DockerStopRemoveCmd(k.ContainerName))
	}

	if err := h.store.DeleteKafka(id, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// KafkaEnvVarName derives the env var name from a Kafka cluster name.
// e.g. "my-kafka" → "MY_KAFKA_URL"
func KafkaEnvVarName(name string) string {
	return DBEnvVarName(name)
}

// KafkaConnectionURL returns the bootstrap server address for the Kafka cluster.
func KafkaConnectionURL(k *models.Kafka) string {
	return fmt.Sprintf("%s:%d", k.NodeHost, k.Port)
}
