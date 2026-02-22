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
	nodeCount, _ := h.store.CountNodes()
	appCount, _ := h.store.CountApplications()
	deploymentCounts, _ := h.store.CountDeploymentsByStatus()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nodes":        nodeCount,
		"applications": appCount,
		"deployments":  deploymentCounts,
	})
}
