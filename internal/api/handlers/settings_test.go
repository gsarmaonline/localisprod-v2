package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gsarma/localisprod-v2/internal/api/handlers"
)

func TestSettingsGet_Empty(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewSettingsHandler(s, "http://localhost:8080")

	rec := httptest.NewRecorder()
	h.Get(rec, getRequest("/api/settings"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	decodeJSON(t, rec, &resp)

	if resp["github_username"] != "" {
		t.Errorf("expected empty github_username, got %q", resp["github_username"])
	}
	if resp["github_token"] != "" {
		t.Errorf("expected empty github_token indicator, got %q", resp["github_token"])
	}
	if resp["webhook_secret"] != "" {
		t.Errorf("expected empty webhook_secret indicator, got %q", resp["webhook_secret"])
	}
}

func TestSettingsGet_WithValues_TokenMasked(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewSettingsHandler(s, "http://localhost:8080")

	_ = s.SetUserSetting(testUserID, "github_username", "myuser")
	_ = s.SetUserSetting(testUserID, "github_token", "ghp_supersecrettoken")
	_ = s.SetUserSetting(testUserID, "webhook_secret", "mysecret")

	rec := httptest.NewRecorder()
	h.Get(rec, getRequest("/api/settings"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	decodeJSON(t, rec, &resp)

	if resp["github_username"] != "myuser" {
		t.Errorf("expected github_username='myuser', got %q", resp["github_username"])
	}
	// Token should be masked as "configured", not exposed.
	if resp["github_token"] != "configured" {
		t.Errorf("expected github_token='configured', got %q", resp["github_token"])
	}
	if resp["webhook_secret"] != "configured" {
		t.Errorf("expected webhook_secret='configured', got %q", resp["webhook_secret"])
	}
}

func TestSettingsUpdate_Success(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewSettingsHandler(s, "http://localhost:8080")

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/settings", map[string]any{
		"github_username": "newuser",
		"github_token":    "newtoken",
		"webhook_secret":  "newsecret",
	})
	r.Method = "PUT"
	h.Update(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body)
	}

	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status='ok', got %q", resp["status"])
	}

	// Verify values are persisted in user settings.
	username, _ := s.GetUserSetting(testUserID, "github_username")
	if username != "newuser" {
		t.Errorf("expected stored username='newuser', got %q", username)
	}
	token, _ := s.GetUserSetting(testUserID, "github_token")
	if token != "newtoken" {
		t.Errorf("expected stored token='newtoken', got %q", token)
	}
	webhookSecret, _ := s.GetUserSetting(testUserID, "webhook_secret")
	if webhookSecret != "newsecret" {
		t.Errorf("expected stored webhook_secret='newsecret', got %q", webhookSecret)
	}
}

func TestSettingsUpdate_EmptyWebhookSecret_NotOverwritten(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewSettingsHandler(s, "http://localhost:8080")

	// Set an existing webhook secret.
	_ = s.SetUserSetting(testUserID, "webhook_secret", "existing-secret")

	// Update with empty webhook_secret — should not overwrite.
	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/settings", map[string]any{
		"github_username": "user",
		"github_token":    "token",
		"webhook_secret":  "",
	})
	r.Method = "PUT"
	h.Update(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Secret should remain unchanged.
	secret, _ := s.GetUserSetting(testUserID, "webhook_secret")
	if secret != "existing-secret" {
		t.Errorf("expected existing secret preserved, got %q", secret)
	}
}

func TestSettingsUpdate_InvalidJSON(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewSettingsHandler(s, "http://localhost:8080")

	rec := httptest.NewRecorder()
	r := withUserID(httptest.NewRequest(http.MethodPut, "/api/settings", nil))
	r.Header.Set("Content-Type", "application/json")
	h.Update(rec, r)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
