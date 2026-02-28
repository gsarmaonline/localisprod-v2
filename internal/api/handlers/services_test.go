package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gsarma/localisprod-v2/internal/api/handlers"
)

func TestServiceCreate_MissingFields(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewServiceHandler(s)

	tests := []struct {
		name string
		body map[string]any
	}{
		{"no fields", map[string]any{}},
		{"missing docker_image", map[string]any{"name": "myapp"}},
		{"missing name", map[string]any{"docker_image": "nginx:latest"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := postJSON(t, "/api/services", tt.body)
			h.Create(rec, r)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d (body: %s)", rec.Code, rec.Body)
			}
		})
	}
}

func TestServiceCreate_InvalidJSON(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewServiceHandler(s)

	rec := httptest.NewRecorder()
	r := withUserID(httptest.NewRequest(http.MethodPost, "/api/services", nil))
	r.Header.Set("Content-Type", "application/json")
	h.Create(rec, r)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestServiceCreate_Success_Minimal(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewServiceHandler(s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/services", map[string]any{
		"name":         "my-app",
		"docker_image": "nginx:latest",
	})
	h.Create(rec, r)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rec.Code, rec.Body)
	}

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if resp["name"] != "my-app" {
		t.Errorf("expected name 'my-app', got %v", resp["name"])
	}
	if resp["docker_image"] != "nginx:latest" {
		t.Errorf("expected docker_image 'nginx:latest', got %v", resp["docker_image"])
	}
	if resp["id"] == nil || resp["id"] == "" {
		t.Error("expected non-empty id")
	}
	// Default JSON values for nil maps/slices.
	if resp["env_vars"] != "{}" {
		t.Errorf("expected env_vars '{}', got %v", resp["env_vars"])
	}
	if resp["ports"] != "[]" {
		t.Errorf("expected ports '[]', got %v", resp["ports"])
	}
}

func TestServiceCreate_Success_WithEnvAndPorts(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewServiceHandler(s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/services", map[string]any{
		"name":         "full-app",
		"docker_image": "myimage:v1",
		"env_vars":     map[string]string{"FOO": "bar", "SECRET": "value"},
		"ports":        []string{"8080:80"},
		"command":      "serve",
		"github_repo":  "owner/repo",
		"domain":       "example.com",
	})
	h.Create(rec, r)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rec.Code, rec.Body)
	}

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if resp["github_repo"] != "owner/repo" {
		t.Errorf("expected github_repo 'owner/repo', got %v", resp["github_repo"])
	}
	if resp["domain"] != "example.com" {
		t.Errorf("expected domain 'example.com', got %v", resp["domain"])
	}
}

func TestServiceList_Empty(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewServiceHandler(s)

	rec := httptest.NewRecorder()
	h.List(rec, getRequest("/api/services"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []any
	decodeJSON(t, rec, &resp)
	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d items", len(resp))
	}
}

func TestServiceList_WithItems(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewServiceHandler(s)
	mustCreateApp(t, s)

	rec := httptest.NewRecorder()
	h.List(rec, getRequest("/api/services"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []map[string]any
	decodeJSON(t, rec, &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 service, got %d", len(resp))
	}
}

func TestServiceGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewServiceHandler(s)

	rec := httptest.NewRecorder()
	h.Get(rec, getRequest("/api/services/nonexistent"), "nonexistent")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if resp["error"] == nil {
		t.Error("expected error field in response")
	}
}

func TestServiceGet_Success(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewServiceHandler(s)
	a := mustCreateApp(t, s)

	rec := httptest.NewRecorder()
	h.Get(rec, getRequest("/api/services/"+a.ID), a.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body)
	}
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if resp["id"] != a.ID {
		t.Errorf("expected id %q, got %v", a.ID, resp["id"])
	}
	if resp["name"] != a.Name {
		t.Errorf("expected name %q, got %v", a.Name, resp["name"])
	}
}

func TestServiceDelete_NotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewServiceHandler(s)

	rec := httptest.NewRecorder()
	r := withUserID(httptest.NewRequest(http.MethodDelete, "/api/services/bad", nil))
	h.Delete(rec, r, "bad")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestServiceDelete_Success(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewServiceHandler(s)
	a := mustCreateApp(t, s)

	rec := httptest.NewRecorder()
	r := withUserID(httptest.NewRequest(http.MethodDelete, "/api/services/"+a.ID, nil))
	h.Delete(rec, r, a.ID)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}

	// Verify deletion.
	rec2 := httptest.NewRecorder()
	h.Get(rec2, getRequest("/api/services/"+a.ID), a.ID)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after deletion, got %d", rec2.Code)
	}
}
