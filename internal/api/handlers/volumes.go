package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/sshexec"
	"github.com/gsarma/localisprod-v2/internal/store"
	"github.com/gsarma/localisprod-v2/internal/volumemigrator"
)

// VolumeHandler handles node volume migration endpoints.
type VolumeHandler struct {
	store    *store.Store
	migrator *volumemigrator.Migrator
}

func NewVolumeHandler(s *store.Store) *VolumeHandler {
	return &VolumeHandler{
		store:    s,
		migrator: volumemigrator.New(s),
	}
}

// Migrate handles POST /api/nodes/:id/volumes/migrate
func (h *VolumeHandler) Migrate(w http.ResponseWriter, r *http.Request, nodeID string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}

	node, err := h.store.GetNodeForUser(nodeID, userID, isRoot(r))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	if node.Provider == "" || node.ProviderInstanceID == "" {
		writeError(w, http.StatusBadRequest, "node must have a provider and provider_instance_id to use block storage migration")
		return
	}
	if node.Provider != "aws" && node.Provider != "digitalocean" {
		writeError(w, http.StatusBadRequest, "unsupported provider: only aws and digitalocean are supported")
		return
	}

	// Check for an existing active migration
	existing, err := h.store.GetVolumeMigration(nodeID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if existing != nil {
		terminal := map[string]bool{
			"completed":   true,
			"rolled_back": true,
			"failed":      true,
		}
		if !terminal[existing.Status] {
			writeError(w, http.StatusConflict, "a migration is already in progress for this node (status: "+existing.Status+")")
			return
		}
	}

	now := time.Now().UTC()
	migration := &models.NodeVolumeMigration{
		ID:        uuid.New().String(),
		NodeID:    nodeID,
		MountPath: "/mnt/localis-data",
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.store.CreateVolumeMigration(migration); err != nil {
		writeInternalError(w, err)
		return
	}

	h.migrator.Migrate(node, migration, userID)

	writeJSON(w, http.StatusAccepted, migration)
}

// GetMigration handles GET /api/nodes/:id/volumes/migration
func (h *VolumeHandler) GetMigration(w http.ResponseWriter, r *http.Request, nodeID string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}

	node, err := h.store.GetNodeForUser(nodeID, userID, isRoot(r))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	migration, err := h.store.GetVolumeMigration(nodeID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if migration == nil {
		writeError(w, http.StatusNotFound, "no migration found for this node")
		return
	}

	writeJSON(w, http.StatusOK, migration)
}

// Rollback handles POST /api/nodes/:id/volumes/rollback
func (h *VolumeHandler) Rollback(w http.ResponseWriter, r *http.Request, nodeID string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}

	node, err := h.store.GetNodeForUser(nodeID, userID, isRoot(r))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	migration, err := h.store.GetVolumeMigration(nodeID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if migration == nil {
		writeError(w, http.StatusNotFound, "no migration found for this node")
		return
	}

	noRollback := map[string]bool{
		"completed":   true,
		"rolled_back": true,
		"rolling_back": true,
	}
	if noRollback[migration.Status] {
		writeError(w, http.StatusConflict, "cannot rollback migration in status: "+migration.Status)
		return
	}

	h.migrator.Rollback(node, migration, userID)

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "rolling_back"})
}

// DeleteBak handles DELETE /api/nodes/:id/volumes/bak
// SSH: rm -rf /var/lib/docker/volumes.bak
func (h *VolumeHandler) DeleteBak(w http.ResponseWriter, r *http.Request, nodeID string) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}

	node, err := h.store.GetNodeForUser(nodeID, userID, isRoot(r))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	migration, err := h.store.GetVolumeMigration(nodeID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if migration == nil || migration.Status != "completed" {
		writeError(w, http.StatusConflict, "can only delete .bak after a completed migration")
		return
	}

	runner := sshexec.NewRunner(node)
	out, runErr := runner.Run("rm -rf /var/lib/docker/volumes.bak")
	if runErr != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "error",
			"output": out + "\n" + runErr.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
