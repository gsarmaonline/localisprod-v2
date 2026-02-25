package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/sshexec"
	"github.com/gsarma/localisprod-v2/internal/store"
)

// localHosts contains addresses that resolve to the local machine.
// Non-root users are blocked from registering nodes with these hosts.
var localHosts = map[string]bool{
	"localhost": true,
	"127.0.0.1": true,
	"::1":       true,
	"0.0.0.0":   true,
}

type NodeHandler struct {
	store *store.Store
}

func NewNodeHandler(s *store.Store) *NodeHandler {
	return &NodeHandler{store: s}
}

func (h *NodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	var body struct {
		Name       string `json:"name"`
		Host       string `json:"host"`
		Port       int    `json:"port"`
		Username   string `json:"username"`
		PrivateKey string `json:"private_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.Host == "" || body.Username == "" || body.PrivateKey == "" {
		writeError(w, http.StatusBadRequest, "name, host, username, and private_key are required")
		return
	}
	if localHosts[strings.ToLower(body.Host)] && !isRoot(r) {
		writeError(w, http.StatusForbidden, "only the root user can register local addresses as nodes; use the management node instead")
		return
	}
	if body.Port == 0 {
		body.Port = 22
	}
	node := &models.Node{
		ID:         uuid.New().String(),
		Name:       body.Name,
		Host:       body.Host,
		Port:       body.Port,
		Username:   body.Username,
		PrivateKey: body.PrivateKey,
		Status:     "unknown",
		CreatedAt:  time.Now().UTC(),
	}
	if err := h.store.CreateNode(node, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	node.PrivateKey = ""
	writeJSON(w, http.StatusCreated, node)
}

func (h *NodeHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	nodes, err := h.store.ListNodes(userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if isRoot(r) {
		if mgmt, err := h.store.GetManagementNode(); err == nil && mgmt != nil {
			mgmt.PrivateKey = ""
			nodes = append([]*models.Node{mgmt}, nodes...)
		}
	}
	for _, n := range nodes {
		n.PrivateKey = ""
	}
	if nodes == nil {
		nodes = []*models.Node{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (h *NodeHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	node, err := h.store.GetNodeForUser(id, userID, isRoot(r))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	node.PrivateKey = ""
	writeJSON(w, http.StatusOK, node)
}

func (h *NodeHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	if id == "management" {
		writeError(w, http.StatusForbidden, "the management node cannot be deleted")
		return
	}
	node, err := h.store.GetNode(id, userID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if err := h.store.DeleteNode(id, userID); err != nil {
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NodeHandler) SetupTraefik(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	node, err := h.store.GetNodeForUser(id, userID, isRoot(r))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	runner := sshexec.NewRunner(node)
	output, runErr := runner.Run(sshexec.TraefikSetupCmd())

	if runErr != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "error",
			"output": output + "\n" + runErr.Error(),
		})
		return
	}

	_ = h.store.UpdateNodeTraefik(id, userID, true)
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"output": output,
	})
}

func (h *NodeHandler) Ping(w http.ResponseWriter, r *http.Request, id string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	node, err := h.store.GetNodeForUser(id, userID, isRoot(r))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	runner := sshexec.NewRunner(node)
	pingErr := runner.Ping()

	status := "online"
	message := "pong"
	if pingErr != nil {
		status = "offline"
		message = pingErr.Error()
	}

	_ = h.store.UpdateNodeStatus(id, userID, status)

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  status,
		"message": message,
	})
}
