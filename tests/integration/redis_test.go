//go:build integration

package integration_test

import (
	"net/http"
	"testing"
	"time"
)

func TestRedisCache(t *testing.T) {
	port := freePort()
	const password = "redispass123"

	// ── Create ────────────────────────────────────────────────────────────────
	resp, err := apiPost("/api/caches", map[string]interface{}{
		"name":     "integ-redis",
		"node_id":  testNodeID,
		"password": password,
		"port":     port,
	})
	if err != nil {
		t.Fatalf("POST /api/caches: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errBody map[string]string
		decodeJSON(t, resp.Body, &errBody)
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, errBody)
	}

	var cache struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		ContainerName string `json:"container_name"`
	}
	decodeJSON(t, resp.Body, &cache)

	if cache.ID == "" {
		t.Fatal("response missing id")
	}
	if cache.Status != "running" {
		t.Fatalf("expected status 'running', got %q", cache.Status)
	}

	t.Cleanup(func() {
		apiDelete("/api/caches/" + cache.ID)
		removeVolume("localisprod-integ-redis-data")
	})

	// ── Verify container is running ───────────────────────────────────────────
	waitForContainer(t, cache.ContainerName, 30*time.Second)
	t.Logf("redis container %q running on port %d", cache.ContainerName, port)

	// ── Ping via redis-cli ────────────────────────────────────────────────────
	var pong bool
	for i := 0; i < 15; i++ {
		out, err := dockerExec(cache.ContainerName,
			"redis-cli", "-a", password, "--no-auth-warning", "ping",
		)
		if err == nil && out == "PONG" {
			t.Logf("redis-cli ping: %s", out)
			pong = true
			break
		}
		time.Sleep(time.Second)
	}
	if !pong {
		t.Fatal("redis did not respond to PING within 15 s")
	}

	// ── SET / GET round-trip ──────────────────────────────────────────────────
	if _, err := dockerExec(cache.ContainerName,
		"redis-cli", "-a", password, "--no-auth-warning",
		"set", "integ-key", "integ-value",
	); err != nil {
		t.Fatalf("redis SET failed: %v", err)
	}
	got, err := dockerExec(cache.ContainerName,
		"redis-cli", "-a", password, "--no-auth-warning",
		"get", "integ-key",
	)
	if err != nil {
		t.Fatalf("redis GET failed: %v", err)
	}
	if got != "integ-value" {
		t.Errorf("expected 'integ-value', got %q", got)
	}

	// ── GET via API ───────────────────────────────────────────────────────────
	getResp, err := apiGet("/api/caches/" + cache.ID)
	if err != nil {
		t.Fatalf("GET /api/caches/%s: %v", cache.ID, err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", getResp.StatusCode)
	}
	var fetched struct {
		Port int `json:"port"`
	}
	decodeJSON(t, getResp.Body, &fetched)
	if fetched.Port != port {
		t.Errorf("expected port %d, got %d", port, fetched.Port)
	}
}
