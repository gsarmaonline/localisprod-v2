//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNginxDeployment(t *testing.T) {
	hostPort := freePort()

	// ── Create service ────────────────────────────────────────────────────────
	appResp, err := apiPost("/api/services", map[string]interface{}{
		"name":         "integ-nginx",
		"docker_image": "nginx:alpine",
		"ports":        []string{fmt.Sprintf("%d:80", hostPort)},
	})
	if err != nil {
		t.Fatalf("POST /api/services: %v", err)
	}
	defer appResp.Body.Close()
	if appResp.StatusCode != http.StatusCreated {
		var e map[string]string
		decodeJSON(t, appResp.Body, &e)
		t.Fatalf("create app expected 201, got %d: %v", appResp.StatusCode, e)
	}
	var app struct {
		ID string `json:"id"`
	}
	decodeJSON(t, appResp.Body, &app)

	t.Cleanup(func() {
		apiDelete("/api/services/" + app.ID)
	})

	// ── Deploy ────────────────────────────────────────────────────────────────
	depResp, err := apiPost("/api/deployments", map[string]interface{}{
		"service_id": app.ID,
		"node_id":    testNodeID,
	})
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer depResp.Body.Close()
	if depResp.StatusCode != http.StatusCreated {
		var e map[string]interface{}
		decodeJSON(t, depResp.Body, &e)
		t.Fatalf("create deployment expected 201, got %d: %v", depResp.StatusCode, e)
	}
	var dep struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		ContainerName string `json:"container_name"`
	}
	decodeJSON(t, depResp.Body, &dep)

	if dep.Status != "running" {
		t.Fatalf("expected deployment status 'running', got %q", dep.Status)
	}

	t.Cleanup(func() {
		apiDelete("/api/deployments/" + dep.ID)
	})

	// ── Verify container is running ───────────────────────────────────────────
	waitForContainer(t, dep.ContainerName, 30*time.Second)
	t.Logf("nginx container %q running, host port %d", dep.ContainerName, hostPort)

	// ── Fetch nginx welcome page ──────────────────────────────────────────────
	var body string
	for i := 0; i < 15; i++ {
		out, err := dockerExec(dep.ContainerName,
			"wget", "-qO-", "http://localhost:80",
		)
		if err == nil && strings.Contains(out, "nginx") {
			body = out
			break
		}
		time.Sleep(time.Second)
	}
	if !strings.Contains(body, "nginx") {
		t.Errorf("nginx welcome page not served — body: %s", body)
	}

	// ── Get logs via API ──────────────────────────────────────────────────────
	logsResp, err := apiGet("/api/deployments/" + dep.ID + "/logs")
	if err != nil {
		t.Fatalf("GET logs: %v", err)
	}
	defer logsResp.Body.Close()
	if logsResp.StatusCode != http.StatusOK {
		t.Fatalf("GET logs expected 200, got %d", logsResp.StatusCode)
	}
	var logs struct {
		Logs string `json:"logs"`
	}
	decodeJSON(t, logsResp.Body, &logs)
	t.Logf("nginx logs snippet: %.200s", logs.Logs)

	// ── Restart via API ───────────────────────────────────────────────────────
	restartResp, err := apiPost("/api/deployments/"+dep.ID+"/restart", nil)
	if err != nil {
		t.Fatalf("POST restart: %v", err)
	}
	defer restartResp.Body.Close()
	if restartResp.StatusCode != http.StatusOK {
		t.Fatalf("restart expected 200, got %d", restartResp.StatusCode)
	}
	var restartBody struct {
		Status string `json:"status"`
	}
	decodeJSON(t, restartResp.Body, &restartBody)
	if restartBody.Status != "running" {
		t.Errorf("restart status expected 'running', got %q", restartBody.Status)
	}

	// Container should still be running after restart.
	waitForContainer(t, dep.ContainerName, 15*time.Second)
}
