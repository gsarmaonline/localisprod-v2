//go:build integration

package integration_test

import (
	"net/http"
	"testing"
	"time"
)

func TestPostgresDatabase(t *testing.T) {
	port := freePort()
	const dbName, dbUser, password = "testdb", "testuser", "testpass123"

	// ── Create ────────────────────────────────────────────────────────────────
	resp, err := apiPost("/api/databases", map[string]interface{}{
		"name":     "integ-pg",
		"type":     "postgres",
		"node_id":  testNodeID,
		"dbname":   dbName,
		"db_user":  dbUser,
		"password": password,
		"port":     port,
	})
	if err != nil {
		t.Fatalf("POST /api/databases: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errBody map[string]string
		decodeJSON(t, resp.Body, &errBody)
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, errBody)
	}

	var db struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		ContainerName string `json:"container_name"`
	}
	decodeJSON(t, resp.Body, &db)

	if db.ID == "" {
		t.Fatal("response missing id")
	}
	if db.Status != "running" {
		t.Fatalf("expected status 'running', got %q", db.Status)
	}

	// Cleanup: DELETE via API (stops + removes the Docker container) then drop volume.
	t.Cleanup(func() {
		apiDelete("/api/databases/" + db.ID)
		removeVolume("localisprod-integ-pg-data")
	})

	// ── Verify container is running ───────────────────────────────────────────
	waitForContainer(t, db.ContainerName, 30*time.Second)
	t.Logf("postgres container %q running on port %d", db.ContainerName, port)

	// ── Wait for postgres to accept connections ───────────────────────────────
	var ready bool
	for i := 0; i < 30; i++ {
		out, err := dockerExec(db.ContainerName, "pg_isready", "-U", dbUser, "-d", dbName)
		if err == nil {
			t.Logf("pg_isready: %s", out)
			ready = true
			break
		}
		time.Sleep(time.Second)
	}
	if !ready {
		t.Fatal("postgres did not become ready within 30 s")
	}

	// ── GET ──────────────────────────────────────────────────────────────────
	getResp, err := apiGet("/api/databases/" + db.ID)
	if err != nil {
		t.Fatalf("GET /api/databases/%s: %v", db.ID, err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", getResp.StatusCode)
	}
	var fetched struct {
		Type string `json:"type"`
		Port int    `json:"port"`
	}
	decodeJSON(t, getResp.Body, &fetched)
	if fetched.Type != "postgres" {
		t.Errorf("expected type 'postgres', got %q", fetched.Type)
	}
	if fetched.Port != port {
		t.Errorf("expected port %d, got %d", port, fetched.Port)
	}

	// ── List ─────────────────────────────────────────────────────────────────
	listResp, err := apiGet("/api/databases")
	if err != nil {
		t.Fatalf("GET /api/databases: %v", err)
	}
	defer listResp.Body.Close()
	var list []struct {
		ID string `json:"id"`
	}
	decodeJSON(t, listResp.Body, &list)
	found := false
	for _, item := range list {
		if item.ID == db.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("database %s not in list response", db.ID)
	}
}
