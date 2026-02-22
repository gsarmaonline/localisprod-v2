package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gsarma/localisprod-v2/internal/store"
)

type SettingsHandler struct {
	store  *store.Store
	appURL string
}

func NewSettingsHandler(s *store.Store, appURL string) *SettingsHandler {
	return &SettingsHandler{store: s, appURL: appURL}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	username, err := h.store.GetUserSetting(userID, "github_username")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	token, err := h.store.GetUserSetting(userID, "github_token")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	webhookSecret, err := h.store.GetUserSetting(userID, "webhook_secret")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	webhookToken, err := h.store.GetUserSetting(userID, "webhook_token")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	tokenStatus := ""
	if token != "" {
		tokenStatus = "configured"
	}
	webhookSecretStatus := ""
	if webhookSecret != "" {
		webhookSecretStatus = "configured"
	}

	webhookURL := ""
	if webhookToken != "" {
		webhookURL = h.appURL + "/api/webhooks/github/" + webhookToken
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"github_username": username,
		"github_token":    tokenStatus,
		"webhook_secret":  webhookSecretStatus,
		"webhook_url":     webhookURL,
	})
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	var body struct {
		GithubUsername string `json:"github_username"`
		GithubToken    string `json:"github_token"`
		WebhookSecret  string `json:"webhook_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.store.SetUserSetting(userID, "github_username", body.GithubUsername); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if body.GithubToken != "" {
		if err := h.store.SetUserSetting(userID, "github_token", body.GithubToken); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if body.WebhookSecret != "" {
		if err := h.store.SetUserSetting(userID, "webhook_secret", body.WebhookSecret); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
