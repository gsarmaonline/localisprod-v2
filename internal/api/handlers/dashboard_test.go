package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gsarma/localisprod-v2/internal/api/handlers"
)

func TestDashboardStats_Empty(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDashboardHandler(s)

	rec := httptest.NewRecorder()
	h.Stats(rec, getRequest("/api/stats"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	decodeJSON(t, rec, &resp)

	if nodes, ok := resp["nodes"].(float64); !ok || nodes != 0 {
		t.Errorf("expected nodes=0, got %v", resp["nodes"])
	}
	if apps, ok := resp["services"].(float64); !ok || apps != 0 {
		t.Errorf("expected services=0, got %v", resp["services"])
	}
	if resp["deployments"] == nil {
		t.Error("expected deployments key in response")
	}
}

func TestDashboardStats_WithData(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDashboardHandler(s)

	n := mustCreateNode(t, s)
	a := mustCreateApp(t, s)
	d := mustCreateDeployment(t, s, a.ID, n.ID)
	_ = s.UpdateDeploymentStatus(d.ID, testUserID, "running", "cid123")

	rec := httptest.NewRecorder()
	h.Stats(rec, getRequest("/api/stats"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	decodeJSON(t, rec, &resp)

	if nodes, ok := resp["nodes"].(float64); !ok || nodes != 1 {
		t.Errorf("expected nodes=1, got %v", resp["nodes"])
	}
	if apps, ok := resp["services"].(float64); !ok || apps != 1 {
		t.Errorf("expected services=1, got %v", resp["services"])
	}

	deplMap, ok := resp["deployments"].(map[string]any)
	if !ok {
		t.Fatalf("expected deployments to be a map, got %T", resp["deployments"])
	}
	if running, ok := deplMap["running"].(float64); !ok || running != 1 {
		t.Errorf("expected deployments.running=1, got %v", deplMap["running"])
	}
}

func TestDashboardStats_ContentType(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewDashboardHandler(s)

	rec := httptest.NewRecorder()
	h.Stats(rec, getRequest("/api/stats"))

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}
