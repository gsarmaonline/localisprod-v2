package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gsarma/localisprod-v2/internal/api/handlers"
)

func TestDeploymentCreate_MissingFields(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)

	tests := []struct {
		name string
		body map[string]any
	}{
		{"no fields", map[string]any{}},
		{"missing node_id", map[string]any{"application_id": "some-id"}},
		{"missing application_id", map[string]any{"node_id": "some-id"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := postJSON(t, "/api/deployments", tt.body)
			h.Create(rec, r)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d (body: %s)", rec.Code, rec.Body)
			}
		})
	}
}

func TestDeploymentCreate_InvalidJSON(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)

	rec := httptest.NewRecorder()
	r := withUserID(httptest.NewRequest(http.MethodPost, "/api/deployments", nil))
	r.Header.Set("Content-Type", "application/json")
	h.Create(rec, r)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestDeploymentCreate_AppNotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/deployments", map[string]any{
		"application_id": "nonexistent-app",
		"node_id":        "some-node",
	})
	h.Create(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d (body: %s)", rec.Code, rec.Body)
	}
}

func TestDeploymentCreate_NodeNotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)
	a := mustCreateApp(t, s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/deployments", map[string]any{
		"application_id": a.ID,
		"node_id":        "nonexistent-node",
	})
	h.Create(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d (body: %s)", rec.Code, rec.Body)
	}
}

func TestDeploymentCreate_LocalNode_NonRoot_Forbidden(t *testing.T) {
	// Non-root users must not be able to deploy to a local (management) node.
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)
	n := mustCreateNode(t, s) // IsLocal=true, owned by testUserID
	a := mustCreateApp(t, s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/deployments", map[string]any{
		"application_id": a.ID,
		"node_id":        n.ID,
	})
	h.Create(rec, r)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d (body: %s)", rec.Code, rec.Body)
	}
}

func TestDeploymentCreate_LocalNode_RootUser_DockerNotAvailable(t *testing.T) {
	// Root users can deploy to local nodes. Docker will likely fail in CI so we
	// accept 200 (docker error details) or 201 (success).
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)
	n := mustCreateNode(t, s) // IsLocal=true
	a := mustCreateApp(t, s)

	rec := httptest.NewRecorder()
	r := postJSONAsRoot(t, "/api/deployments", map[string]any{
		"application_id": a.ID,
		"node_id":        n.ID,
	})
	h.Create(rec, r)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Errorf("expected 200 or 201, got %d (body: %s)", rec.Code, rec.Body)
	}
}

func TestDeploymentList_Empty(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)

	rec := httptest.NewRecorder()
	h.List(rec, getRequest("/api/deployments"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []any
	decodeJSON(t, rec, &resp)
	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d items", len(resp))
	}
}

func TestDeploymentList_WithItems(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)
	n := mustCreateNode(t, s)
	a := mustCreateApp(t, s)
	mustCreateDeployment(t, s, a.ID, n.ID)

	rec := httptest.NewRecorder()
	h.List(rec, getRequest("/api/deployments"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []map[string]any
	decodeJSON(t, rec, &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(resp))
	}
	if resp[0]["status"] != "running" {
		t.Errorf("expected status=running, got %v", resp[0]["status"])
	}
}

func TestDeploymentGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)

	rec := httptest.NewRecorder()
	h.Get(rec, getRequest("/api/deployments/nonexistent"), "nonexistent")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestDeploymentGet_Success(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)
	n := mustCreateNode(t, s)
	a := mustCreateApp(t, s)
	d := mustCreateDeployment(t, s, a.ID, n.ID)

	rec := httptest.NewRecorder()
	h.Get(rec, getRequest("/api/deployments/"+d.ID), d.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body)
	}
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if resp["id"] != d.ID {
		t.Errorf("expected id %q, got %v", d.ID, resp["id"])
	}
	// Joined fields should be present.
	if resp["app_name"] == nil || resp["app_name"] == "" {
		t.Error("expected app_name from join")
	}
	if resp["node_name"] == nil || resp["node_name"] == "" {
		t.Error("expected node_name from join")
	}
}

func TestDeploymentDelete_NotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)

	rec := httptest.NewRecorder()
	r := withUserID(httptest.NewRequest(http.MethodDelete, "/api/deployments/bad", nil))
	h.Delete(rec, r, "bad")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestDeploymentDelete_Success(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)
	n := mustCreateNode(t, s) // local node — docker stop may fail silently
	a := mustCreateApp(t, s)
	d := mustCreateDeployment(t, s, a.ID, n.ID)

	rec := httptest.NewRecorder()
	r := withUserID(httptest.NewRequest(http.MethodDelete, "/api/deployments/"+d.ID, nil))
	h.Delete(rec, r, d.ID)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d (body: %s)", rec.Code, rec.Body)
	}

	// Verify it is gone from the store.
	rec2 := httptest.NewRecorder()
	h.Get(rec2, getRequest("/api/deployments/"+d.ID), d.ID)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after deletion, got %d", rec2.Code)
	}
}

func TestDeploymentRestart_NotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/deployments/bad/restart", nil)
	h.Restart(rec, r, "bad")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestDeploymentRestart_NodeMissing(t *testing.T) {
	// Create a deployment whose node is subsequently deleted.
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)
	n := mustCreateNode(t, s)
	a := mustCreateApp(t, s)
	d := mustCreateDeployment(t, s, a.ID, n.ID)

	// Delete the node.
	_ = s.DeleteNode(n.ID, testUserID)

	rec := httptest.NewRecorder()
	r := postJSON(t, "/api/deployments/"+d.ID+"/restart", nil)
	h.Restart(rec, r, d.ID)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when node is missing, got %d", rec.Code)
	}
}

func TestDeploymentLogs_NotFound(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)

	rec := httptest.NewRecorder()
	h.Logs(rec, getRequest("/api/deployments/bad/logs"), "bad")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestDeploymentLogs_NodeMissing(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDeploymentHandler(s)
	n := mustCreateNode(t, s)
	a := mustCreateApp(t, s)
	d := mustCreateDeployment(t, s, a.ID, n.ID)
	_ = s.DeleteNode(n.ID, testUserID)

	rec := httptest.NewRecorder()
	h.Logs(rec, getRequest("/api/deployments/"+d.ID+"/logs"), d.ID)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when node missing, got %d", rec.Code)
	}
}
