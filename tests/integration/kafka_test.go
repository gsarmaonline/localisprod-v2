//go:build integration

package integration_test

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestKafkaCluster(t *testing.T) {
	port := freePort()

	// ── Create ────────────────────────────────────────────────────────────────
	resp, err := apiPost("/api/kafkas", map[string]interface{}{
		"name":    "integ-kafka",
		"node_id": testNodeID,
		"port":    port,
	})
	if err != nil {
		t.Fatalf("POST /api/kafkas: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errBody map[string]interface{}
		decodeJSON(t, resp.Body, &errBody)
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, errBody)
	}

	var kafka struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		ContainerName string `json:"container_name"`
	}
	decodeJSON(t, resp.Body, &kafka)

	if kafka.ID == "" {
		t.Fatal("response missing id")
	}
	if kafka.Status != "running" {
		t.Fatalf("expected status 'running', got %q", kafka.Status)
	}

	t.Cleanup(func() {
		apiDelete("/api/kafkas/" + kafka.ID)
		removeVolume("localisprod-kafka-integ-kafka-data")
	})

	// ── Verify container is running ───────────────────────────────────────────
	waitForContainer(t, kafka.ContainerName, 60*time.Second)
	t.Logf("kafka container %q running on port %d", kafka.ContainerName, port)

	// ── Wait for broker to log "Kafka Server started" ────────────────────────
	// kafka-topics.sh cannot be used from inside the container: Kafka advertises
	// its host-mapped port in metadata responses, so clients inside the container
	// would try to reconnect to that port which is not bound inside the container.
	// Reading broker logs is simpler and reliable.
	var brokerReady bool
	for i := 0; i < 60; i++ {
		out, err := exec.Command("docker", "logs", kafka.ContainerName).CombinedOutput()
		if err == nil && strings.Contains(string(out), "Kafka Server started") {
			t.Logf("kafka broker ready after ~%d s", i*2)
			brokerReady = true
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !brokerReady {
		t.Fatal("kafka broker did not start within 120 s")
	}

	// ── GET via API ───────────────────────────────────────────────────────────
	getResp, err := apiGet("/api/kafkas/" + kafka.ID)
	if err != nil {
		t.Fatalf("GET /api/kafkas/%s: %v", kafka.ID, err)
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
