package handlers_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gsarma/localisprod-v2/internal/api/handlers"
	"github.com/gsarma/localisprod-v2/internal/models"
)

// signBody computes the X-Hub-Signature-256 header value for a payload.
func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func webhookRequest(t *testing.T, event string, payload any, secret string) *http.Request {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-GitHub-Event", event)
	if secret != "" {
		r.Header.Set("X-Hub-Signature-256", signBody(secret, body))
	}
	return r
}

// ---- verifySignature (indirectly tested through the handler) ----

func TestWebhookGithub_NoSecret_IgnoreNonRegistryEvent(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	payload := map[string]any{"action": "opened"}
	rec := httptest.NewRecorder()
	r := webhookRequest(t, "push", payload, "")
	h.Github(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["status"] != "ignored" {
		t.Errorf("expected status=ignored, got %q", resp["status"])
	}
	if resp["event"] != "push" {
		t.Errorf("expected event=push, got %q", resp["event"])
	}
}

func TestWebhookGithub_InvalidSignature(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)
	_ = s.SetSetting("webhook_secret", "mysecret")

	payload := map[string]any{"action": "published"}
	body, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", bytes.NewReader(body))
	r.Header.Set("X-GitHub-Event", "registry_package")
	r.Header.Set("X-Hub-Signature-256", "sha256=badhex1234")
	h.Github(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d (body: %s)", rec.Code, rec.Body)
	}
}

func TestWebhookGithub_MissingSignature_WhenSecretConfigured(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)
	_ = s.SetSetting("webhook_secret", "mysecret")

	payload := map[string]any{"action": "published"}
	body, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", bytes.NewReader(body))
	r.Header.Set("X-GitHub-Event", "registry_package")
	// No X-Hub-Signature-256 header.
	h.Github(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestWebhookGithub_ValidSignature_RegistryEvent_NoMatchingApp(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)
	_ = s.SetSetting("webhook_secret", "mysecret")

	payload := map[string]any{
		"action": "published",
		"repository": map[string]any{
			"full_name": "owner/no-such-repo",
		},
	}

	rec := httptest.NewRecorder()
	r := webhookRequest(t, "registry_package", payload, "mysecret")
	h.Github(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body)
	}
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if redeployed, ok := resp["redeployed"].(float64); !ok || redeployed != 0 {
		t.Errorf("expected redeployed=0, got %v", resp["redeployed"])
	}
}

func TestWebhookGithub_EmptyRepoName(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	payload := map[string]any{
		"action":     "published",
		"repository": map[string]any{"full_name": ""},
	}

	rec := httptest.NewRecorder()
	r := webhookRequest(t, "registry_package", payload, "")
	h.Github(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if redeployed, ok := resp["redeployed"].(float64); !ok || redeployed != 0 {
		t.Errorf("expected redeployed=0, got %v", resp["redeployed"])
	}
}

func TestWebhookGithub_InvalidJSON(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", bytes.NewReader([]byte("not json")))
	r.Header.Set("X-GitHub-Event", "registry_package")
	h.Github(rec, r)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestWebhookGithub_WithMatchingApp_NonRunningDeployment(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	// Create a local node, app, and a pending (non-running) deployment.
	n := &models.Node{
		ID:        "wh-node",
		Name:      "wh-node",
		Host:      "127.0.0.1",
		Port:      22,
		Username:  "root",
		IsLocal:   true,
		Status:    "online",
		CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateNode(n)

	app := &models.Application{
		ID:          "wh-app",
		Name:        "wh-app",
		DockerImage: "nginx:latest",
		EnvVars:     `{}`,
		Ports:       `[]`,
		GithubRepo:  "owner/my-repo",
		CreatedAt:   time.Now().UTC(),
	}
	_ = s.CreateApplication(app)

	dep := &models.Deployment{
		ID:            "wh-dep",
		ApplicationID: app.ID,
		NodeID:        n.ID,
		ContainerName: "localisprod-wh-app-abcd1234",
		Status:        "pending", // not running — should be skipped
		CreatedAt:     time.Now().UTC(),
	}
	_ = s.CreateDeployment(dep)

	payload := map[string]any{
		"action": "published",
		"repository": map[string]any{
			"full_name": "owner/my-repo",
		},
	}

	rec := httptest.NewRecorder()
	r := webhookRequest(t, "registry_package", payload, "")
	h.Github(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body)
	}
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	// Non-running deployment should be skipped.
	if redeployed, _ := resp["redeployed"].(float64); redeployed != 0 {
		t.Errorf("expected redeployed=0 for non-running deployment, got %v", redeployed)
	}
}

// ---- Signature verification (via header logic) ----

func TestWebhookGithub_SignatureVerification_WrongSecret(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)
	_ = s.SetSetting("webhook_secret", "correct-secret")

	payload := map[string]any{"action": "published", "repository": map[string]any{"full_name": "owner/repo"}}
	body, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", bytes.NewReader(body))
	r.Header.Set("X-GitHub-Event", "registry_package")
	// Sign with a different secret.
	r.Header.Set("X-Hub-Signature-256", signBody("wrong-secret", body))
	h.Github(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong secret, got %d", rec.Code)
	}
}

func TestWebhookGithub_SignatureVerification_ValidHMAC(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)
	secret := "my-webhook-secret"
	_ = s.SetSetting("webhook_secret", secret)

	payload := map[string]any{
		"action":     "published",
		"repository": map[string]any{"full_name": "owner/norepo"},
	}
	body, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", bytes.NewReader(body))
	r.Header.Set("X-GitHub-Event", "registry_package")
	r.Header.Set("X-Hub-Signature-256", signBody(secret, body))
	h.Github(rec, r)

	// Signature valid → proceeds to process event (no matching apps → redeployed=0).
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with valid HMAC, got %d (body: %s)", rec.Code, rec.Body)
	}
}

func TestWebhookGithub_PingEvent_Ignored(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	payload := map[string]any{"zen": "Keep it logically awesome."}
	rec := httptest.NewRecorder()
	r := webhookRequest(t, "ping", payload, "")
	h.Github(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for ping, got %d", rec.Code)
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["status"] != "ignored" {
		t.Errorf("expected status=ignored for ping event, got %q", resp["status"])
	}
}

// ---- Signature edge cases ----

func TestWebhookGithub_SignatureNoPrefix(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)
	_ = s.SetSetting("webhook_secret", "secret")

	payload := map[string]any{}
	body, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", bytes.NewReader(body))
	r.Header.Set("X-GitHub-Event", "registry_package")
	// Signature without the "sha256=" prefix.
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	r.Header.Set("X-Hub-Signature-256", hex.EncodeToString(mac.Sum(nil)))
	h.Github(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when prefix missing, got %d", rec.Code)
	}
}

func TestWebhookGithub_ResponseIncludesRepo(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	payload := map[string]any{
		"action": "published",
		"repository": map[string]any{
			"full_name": "owner/my-repo",
		},
	}
	rec := httptest.NewRecorder()
	r := webhookRequest(t, "registry_package", payload, "")
	h.Github(rec, r)

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if resp["repo"] != "owner/my-repo" {
		t.Errorf("expected repo='owner/my-repo' in response, got %v", resp["repo"])
	}
}

// Ensure the handler can process many different event types gracefully.
func TestWebhookGithub_VariousIgnoredEvents(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	events := []string{"push", "pull_request", "issues", "release", "star", "fork"}
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			payload := map[string]any{"action": "created"}
			rec := httptest.NewRecorder()
			r := webhookRequest(t, event, payload, "")
			h.Github(rec, r)

			if rec.Code != http.StatusOK {
				t.Errorf("event %q: expected 200, got %d", event, rec.Code)
			}
			var resp map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("event %q: decode: %v", event, err)
			}
			if resp["status"] != "ignored" {
				t.Errorf("event %q: expected status=ignored, got %q", event, resp["status"])
			}
		})
	}
}

// Ensure Content-Type is set on all responses.
func TestWebhookGithub_ContentType(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	payload := map[string]any{"action": "published"}
	rec := httptest.NewRecorder()
	r := webhookRequest(t, "push", payload, "")
	h.Github(rec, r)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

// Test that large body doesn't panic.
func TestWebhookGithub_LargePayload(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewWebhookHandler(s)

	// Build a payload with a large field.
	payload := map[string]any{
		"action":  "published",
		"big_str": fmt.Sprintf("%0*d", 10000, 0),
		"repository": map[string]any{
			"full_name": "owner/repo",
		},
	}
	rec := httptest.NewRecorder()
	r := webhookRequest(t, "registry_package", payload, "")
	h.Github(rec, r)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for large payload, got %d", rec.Code)
	}
}
