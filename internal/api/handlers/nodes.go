package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/sshexec"
	"github.com/gsarma/localisprod-v2/internal/store"
)

type NodeHandler struct {
	store *store.Store
}

func NewNodeHandler(s *store.Store) *NodeHandler {
	return &NodeHandler{store: s}
}

func (h *NodeHandler) Create(w http.ResponseWriter, r *http.Request) {
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
	if err := h.store.CreateNode(node); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	node.PrivateKey = ""
	writeJSON(w, http.StatusCreated, node)
}

func (h *NodeHandler) List(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.store.ListNodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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
	node, err := h.store.GetNode(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
	node, err := h.store.GetNode(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if err := h.store.DeleteNode(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NodeHandler) Ping(w http.ResponseWriter, r *http.Request, id string) {
	node, err := h.store.GetNode(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	client := sshexec.New(node.Host, node.Port, node.Username, node.PrivateKey)
	pingErr := client.Ping()

	status := "online"
	message := "pong"
	if pingErr != nil {
		status = "offline"
		message = pingErr.Error()
	}

	_ = h.store.UpdateNodeStatus(id, status)

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  status,
		"message": message,
	})
}
