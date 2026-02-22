package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gsarma/localisprod-v2/internal/api/handlers"
)

func TestNodeCreate_MissingFields(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	tests := []struct {
		name string
		body map[string]any
	}{
		{"no fields", map[string]any{}},
		{"missing host", map[string]any{"name": "n", "username": "u", "private_key": "k"}},
		{"missing name", map[string]any{"host": "h", "username": "u", "private_key": "k"}},
		{"missing username", map[string]any{"name": "n", "host": "h", "private_key": "k"}},
		{"missing private_key", map[string]any{"name": "n", "host": "h", "username": "u"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := postJSON(t, "/api/nodes", tt.body)
			h.Create(rec, r)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d (body: %s)", rec.Code, rec.Body)
			}
		})
	}
}

func TestNodeCreate_InvalidJSON(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/nodes", nil)
	r.Header.Set("Content-Type", "application/json")
	h.Create(rec, r)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestNodeCreate_Success(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/nodes", map[string]any{
		"name":        "my-server",
		"host":        "10.0.0.1",
		"username":    "ubuntu",
		"private_key": "-----BEGIN RSA PRIVATE KEY-----\nfake\n-----END RSA PRIVATE KEY-----",
	})
	h.Create(rec, r)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rec.Code, rec.Body)
	}

	var resp map[string]any
	decodeJSON(t, rec, &resp)

	if resp["name"] != "my-server" {
		t.Errorf("expected name 'my-server', got %v", resp["name"])
	}
	if resp["private_key"] != nil && resp["private_key"] != "" {
		t.Error("private_key must not be returned in response")
	}
	if resp["id"] == nil || resp["id"] == "" {
		t.Error("expected non-empty id in response")
	}
	// Port should default to 22.
	if port, ok := resp["port"].(float64); !ok || port != 22 {
		t.Errorf("expected default port 22, got %v", resp["port"])
	}
}

func TestNodeCreate_DefaultPort(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/nodes", map[string]any{
		"name":        "server",
		"host":        "1.2.3.4",
		"username":    "root",
		"private_key": "k",
	})
	h.Create(rec, r)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if port, _ := resp["port"].(float64); port != 22 {
		t.Errorf("expected port 22, got %v", resp["port"])
	}
}

func TestNodeCreate_ExplicitPort(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/nodes", map[string]any{
		"name":        "server",
		"host":        "1.2.3.4",
		"port":        2222,
		"username":    "root",
		"private_key": "k",
	})
	h.Create(rec, r)
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if port, _ := resp["port"].(float64); port != 2222 {
		t.Errorf("expected port 2222, got %v", resp["port"])
	}
}

func TestNodeList_Empty(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	rec := httptest.NewRecorder()
	h.List(rec, getRequest("/api/nodes"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []any
	decodeJSON(t, rec, &resp)
	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d items", len(resp))
	}
}

func TestNodeList_WithNodes(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	mustCreateNode(t, s)

	rec := httptest.NewRecorder()
	h.List(rec, getRequest("/api/nodes"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []map[string]any
	decodeJSON(t, rec, &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 node, got %d", len(resp))
	}
	// private_key should be stripped.
	if pk, exists := resp[0]["private_key"]; exists && pk != "" {
		t.Error("private_key must be stripped from list response")
	}
}

func TestNodeGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	rec := httptest.NewRecorder()
	h.Get(rec, getRequest("/api/nodes/nonexistent"), "nonexistent")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestNodeGet_Success(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)
	n := mustCreateNode(t, s)

	rec := httptest.NewRecorder()
	h.Get(rec, getRequest("/api/nodes/"+n.ID), n.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body)
	}
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if resp["id"] != n.ID {
		t.Errorf("expected id %q, got %v", n.ID, resp["id"])
	}
	// private_key should be stripped.
	if pk, exists := resp["private_key"]; exists && pk != "" {
		t.Error("private_key must be stripped from get response")
	}
}

func TestNodeDelete_NotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/nodes/nonexistent", nil)
	h.Delete(rec, r, "nonexistent")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestNodeDelete_Success(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)
	n := mustCreateNode(t, s)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/nodes/"+n.ID, nil)
	h.Delete(rec, r, n.ID)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}

	// Verify deletion.
	rec2 := httptest.NewRecorder()
	h.Get(rec2, getRequest("/api/nodes/"+n.ID), n.ID)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after deletion, got %d", rec2.Code)
	}
}

func TestNodePing_NotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/nodes/badid/ping", nil)
	h.Ping(rec, r, "badid")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestNodePing_LocalNode(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)
	n := mustCreateNode(t, s) // IsLocal=true

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/nodes/"+n.ID+"/ping", nil)
	h.Ping(rec, r, n.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body)
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["status"] != "online" {
		t.Errorf("expected status=online, got %q", resp["status"])
	}
}

func TestNodeSetupTraefik_NotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewNodeHandler(s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/nodes/badid/setup-traefik", nil)
	h.SetupTraefik(rec, r, "badid")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
