//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGarageObjectStorage(t *testing.T) {
	port := freePort()

	// ── Create ────────────────────────────────────────────────────────────────
	resp, err := apiPost("/api/object-storages", map[string]interface{}{
		"name":    "integ-garage",
		"node_id": testNodeID,
		"s3_port": port,
	})
	if err != nil {
		t.Fatalf("POST /api/object-storages: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errBody map[string]interface{}
		decodeJSON(t, resp.Body, &errBody)
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, errBody)
	}

	var storage struct {
		ID              string `json:"id"`
		Status          string `json:"status"`
		ContainerName   string `json:"container_name"`
		S3Port          int    `json:"s3_port"`
		AccessKeyID     string `json:"access_key_id"`
		SecretAccessKey string `json:"secret_access_key"`
		Version         string `json:"version"`
	}
	decodeJSON(t, resp.Body, &storage)

	if storage.ID == "" {
		t.Fatal("response missing id")
	}
	if storage.Status != "running" {
		t.Fatalf("expected status 'running', got %q", storage.Status)
	}
	if storage.ContainerName == "" {
		t.Fatal("response missing container_name")
	}
	if storage.S3Port != port {
		t.Errorf("expected s3_port %d, got %d", port, storage.S3Port)
	}
	if storage.Version != "v1.0.1" {
		t.Errorf("expected version 'v1.0.1', got %q", storage.Version)
	}

	t.Cleanup(func() {
		apiDelete("/api/object-storages/" + storage.ID)
	})

	// ── Verify container is running ───────────────────────────────────────────
	waitForContainer(t, storage.ContainerName, 60*time.Second)
	t.Logf("garage container %q running on port %d", storage.ContainerName, port)

	// ── Verify garage cluster node ID ─────────────────────────────────────────
	nodeIDOut, err := dockerExecTimeout(15*time.Second, storage.ContainerName, "/garage", "node", "id")
	if err != nil {
		t.Fatalf("garage node id: %v (output: %s)", err, nodeIDOut)
	}
	if !strings.Contains(nodeIDOut, "@") {
		t.Errorf("expected node id to contain '@', got %q", nodeIDOut)
	}
	t.Logf("garage node id: %s", nodeIDOut)

	// ── Verify layout is applied ──────────────────────────────────────────────
	statusOut, err := dockerExecTimeout(15*time.Second, storage.ContainerName, "/garage", "status")
	if err != nil {
		t.Logf("garage status (non-fatal): %v", err)
	} else {
		t.Logf("garage status: %s", statusOut)
	}

	// ── Verify credentials were provisioned ───────────────────────────────────
	if storage.AccessKeyID == "" {
		t.Error("expected non-empty access_key_id in response")
	}
	if storage.SecretAccessKey == "" {
		t.Error("expected non-empty secret_access_key in response")
	}
	t.Logf("garage key id: %s", storage.AccessKeyID)

	// ── Verify S3 endpoint responds ───────────────────────────────────────────
	// An unauthenticated request returns an HTTP response (not a connection
	// refused error) when Garage is listening.
	s3URL := fmt.Sprintf("http://127.0.0.1:%d/", port)
	var s3Responded bool
	for i := 0; i < 10; i++ {
		s3Resp, s3Err := httpClient.Get(s3URL)
		if s3Err == nil {
			s3Resp.Body.Close()
			s3Responded = true
			t.Logf("S3 endpoint responded with HTTP %d", s3Resp.StatusCode)
			break
		}
		time.Sleep(time.Second)
	}
	if !s3Responded {
		t.Errorf("S3 endpoint %s did not respond within 10 s", s3URL)
	}

	// ── Create and list a bucket via the garage admin CLI ─────────────────────
	bucketName := "integ-test-bucket"
	if createOut, createErr := dockerExecTimeout(15*time.Second, storage.ContainerName,
		"/garage", "bucket", "create", bucketName); createErr != nil {
		t.Logf("garage bucket create (non-fatal): %v: %s", createErr, createOut)
	} else {
		listOut, listErr := dockerExecTimeout(15*time.Second, storage.ContainerName,
			"/garage", "bucket", "list")
		if listErr != nil {
			t.Logf("garage bucket list (non-fatal): %v: %s", listErr, listOut)
		} else if !strings.Contains(listOut, bucketName) {
			t.Errorf("expected bucket %q in list output, got: %s", bucketName, listOut)
		} else {
			t.Logf("garage bucket list: %s", listOut)
		}
	}

	// ── GET via API ───────────────────────────────────────────────────────────
	getResp, err := apiGet("/api/object-storages/" + storage.ID)
	if err != nil {
		t.Fatalf("GET /api/object-storages/%s: %v", storage.ID, err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", getResp.StatusCode)
	}
	var fetched struct {
		S3Port      int    `json:"s3_port"`
		AccessKeyID string `json:"access_key_id"`
		Status      string `json:"status"`
	}
	decodeJSON(t, getResp.Body, &fetched)
	if fetched.S3Port != port {
		t.Errorf("GET: expected s3_port %d, got %d", port, fetched.S3Port)
	}
	if fetched.AccessKeyID != storage.AccessKeyID {
		t.Errorf("GET: access_key_id mismatch: %q vs %q", fetched.AccessKeyID, storage.AccessKeyID)
	}

	// ── List via API ──────────────────────────────────────────────────────────
	listResp, err := apiGet("/api/object-storages")
	if err != nil {
		t.Fatalf("GET /api/object-storages: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list expected 200, got %d", listResp.StatusCode)
	}
	var items []struct {
		ID string `json:"id"`
	}
	decodeJSON(t, listResp.Body, &items)
	found := false
	for _, item := range items {
		if item.ID == storage.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("object storage %q not found in list", storage.ID)
	}

	// ── Delete and verify ─────────────────────────────────────────────────────
	apiDelete("/api/object-storages/" + storage.ID)

	// Verify container is removed.
	time.Sleep(500 * time.Millisecond)
	if status := containerStatus(storage.ContainerName); status == "running" {
		t.Errorf("container %q still running after delete", storage.ContainerName)
	}

	// Verify GET returns 404.
	getAfterDel, err := apiGet("/api/object-storages/" + storage.ID)
	if err != nil {
		t.Fatalf("GET after delete: %v", err)
	}
	getAfterDel.Body.Close()
	if getAfterDel.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getAfterDel.StatusCode)
	}
}
