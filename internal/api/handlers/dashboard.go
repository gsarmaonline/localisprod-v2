package handlers

import (
	"net/http"

	"github.com/gsarma/localisprod-v2/internal/store"
)

type DashboardHandler struct {
	store *store.Store
}

func NewDashboardHandler(s *store.Store) *DashboardHandler {
	return &DashboardHandler{store: s}
}

func (h *DashboardHandler) Stats(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	nodeCount, _ := h.store.CountNodes(userID)
	serviceCount, _ := h.store.CountServices(userID)
	deploymentCounts, _ := h.store.CountDeploymentsByStatus(userID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nodes":       nodeCount,
		"services":    serviceCount,
		"deployments": deploymentCounts,
	})
}
