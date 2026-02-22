package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gsarma/localisprod-v2/internal/store"
)

type SettingsHandler struct {
	store *store.Store
}

func NewSettingsHandler(s *store.Store) *SettingsHandler {
	return &SettingsHandler{store: s}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	username, err := h.store.GetSetting("github_username")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	token, err := h.store.GetSetting("github_token")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	tokenStatus := ""
	if token != "" {
		tokenStatus = "configured"
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"github_username": username,
		"github_token":    tokenStatus,
	})
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GithubUsername string `json:"github_username"`
		GithubToken    string `json:"github_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.store.SetSetting("github_username", body.GithubUsername); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.store.SetSetting("github_token", body.GithubToken); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
