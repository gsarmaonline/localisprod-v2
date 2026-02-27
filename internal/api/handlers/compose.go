package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gsarma/localisprod-v2/internal/compose"
	"github.com/gsarma/localisprod-v2/internal/store"
)

type ComposeHandler struct {
	store *store.Store
}

func NewComposeHandler(s *store.Store) *ComposeHandler {
	return &ComposeHandler{store: s}
}

// Preview parses a docker-compose.yml and returns classified service objects.
// POST /api/import/docker-compose
func (h *ComposeHandler) Preview(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	preview, err := compose.Parse([]byte(body.Content))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, preview)
}
