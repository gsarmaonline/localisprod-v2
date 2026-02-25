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
	doToken, err := h.store.GetUserSetting(userID, "do_api_token")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	awsKeyID, err := h.store.GetUserSetting(userID, "aws_access_key_id")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	awsSecret, err := h.store.GetUserSetting(userID, "aws_secret_access_key")
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
	doTokenStatus := ""
	if doToken != "" {
		doTokenStatus = "configured"
	}
	awsSecretStatus := ""
	if awsSecret != "" {
		awsSecretStatus = "configured"
	}

	webhookURL := ""
	if webhookToken != "" {
		webhookURL = h.appURL + "/api/webhooks/github/" + webhookToken
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"github_username":       username,
		"github_token":          tokenStatus,
		"webhook_secret":        webhookSecretStatus,
		"webhook_url":           webhookURL,
		"do_api_token":          doTokenStatus,
		"aws_access_key_id":     awsKeyID,
		"aws_secret_access_key": awsSecretStatus,
	})
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(w, r)
	if userID == "" {
		return
	}
	var body struct {
		GithubUsername      string `json:"github_username"`
		GithubToken         string `json:"github_token"`
		WebhookSecret       string `json:"webhook_secret"`
		DOAPIToken          string `json:"do_api_token"`
		AWSAccessKeyID      string `json:"aws_access_key_id"`
		AWSSecretAccessKey  string `json:"aws_secret_access_key"`
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
	if body.DOAPIToken != "" {
		if err := h.store.SetUserSetting(userID, "do_api_token", body.DOAPIToken); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if body.AWSAccessKeyID != "" {
		if err := h.store.SetUserSetting(userID, "aws_access_key_id", body.AWSAccessKeyID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if body.AWSSecretAccessKey != "" {
		if err := h.store.SetUserSetting(userID, "aws_secret_access_key", body.AWSSecretAccessKey); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
