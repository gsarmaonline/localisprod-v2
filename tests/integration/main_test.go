//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	internalapi "github.com/gsarma/localisprod-v2/internal/api"
	"github.com/gsarma/localisprod-v2/internal/auth"
	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/store"
)

const (
	testUserID    = "integration-test-user"
	testJWTSecret = "integration-test-jwt-secret-32b!!"
	testRootEmail = "root@integration.test"
)

var (
	serverURL     string
	sessionCookie string
	testNodeID    string
	httpClient    = &http.Client{Timeout: 90 * time.Second}
)

func TestMain(m *testing.M) {
	if err := checkDocker(); err != nil {
		fmt.Fprintf(os.Stderr, "skipping integration tests: %v\n", err)
		os.Exit(0)
	}

	stopServer, err := startServer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start server: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// Global safety-net: force-remove any leftover localisprod containers and volumes
	// in case individual test cleanups failed.
	globalCleanup()
	stopServer()
	os.Exit(code)
}

func checkDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found in PATH")
	}
	if out, err := exec.Command("docker", "info").CombinedOutput(); err != nil {
		return fmt.Errorf("docker daemon not available: %s", string(out))
	}
	return nil
}

func startServer() (func(), error) {
	dbFile, err := os.CreateTemp("", "localisprod-integration-*.db")
	if err != nil {
		return nil, fmt.Errorf("create temp db: %w", err)
	}
	dbFile.Close()
	dbPath := dbFile.Name()

	// nil cipher = plaintext storage; fine for tests since the data is ephemeral.
	s, err := store.New(dbPath, nil)
	if err != nil {
		os.Remove(dbPath)
		return nil, fmt.Errorf("store.New: %w", err)
	}

	// Insert a local node directly — the Create API does not expose is_local,
	// but NewRunner returns a LocalRunner when IsLocal=true, which runs docker
	// commands via sh without needing SSH.
	localNode := &models.Node{
		ID:         "integration-local-node",
		Name:       "local",
		Host:       "localhost",
		Port:       22,
		Username:   "root",
		PrivateKey: "unused-for-local-runner",
		Status:     "unknown",
		IsLocal:    true,
	}
	if err := s.CreateNode(localNode, testUserID); err != nil {
		os.Remove(dbPath)
		return nil, fmt.Errorf("create local node: %w", err)
	}
	testNodeID = localNode.ID

	// Mint a root JWT with the test secret.  The JWT middleware validates it
	// just like a real Google-issued session, so auth is fully exercised.
	jwtSvc := auth.NewJWTService(testJWTSecret)
	token, err := jwtSvc.Issue(testUserID, testRootEmail, "Integration Root", "", true)
	if err != nil {
		os.Remove(dbPath)
		return nil, fmt.Errorf("mint JWT: %w", err)
	}
	sessionCookie = token

	// oauthSvc is nil — we never call /api/auth/google* in integration tests.
	handler := internalapi.NewRouter(s, nil, jwtSvc, "http://localhost", testRootEmail)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Remove(dbPath)
		return nil, fmt.Errorf("listen: %w", err)
	}
	serverURL = "http://" + ln.Addr().String()

	srv := &http.Server{Handler: handler}
	go func() { _ = srv.Serve(ln) }()

	// Wait until the server responds (up to 2 s).
	probe := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 20; i++ {
		req, _ := http.NewRequest(http.MethodGet, serverURL+"/api/stats", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: token})
		if resp, err := probe.Do(req); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return func() {
		srv.Close()
		os.Remove(dbPath)
	}, nil
}

func globalCleanup() {
	// Stop and remove every container whose name starts with "localisprod".
	out, err := exec.Command("docker", "ps", "-aq", "--filter", "name=localisprod").Output()
	if err == nil {
		ids := strings.Fields(string(out))
		if len(ids) > 0 {
			args := append([]string{"rm", "-f"}, ids...)
			exec.Command("docker", args...).Run()
		}
	}

	// Remove every volume whose name starts with "localisprod".
	out, err = exec.Command("docker", "volume", "ls", "-q", "--filter", "name=localisprod").Output()
	if err == nil {
		vols := strings.Fields(string(out))
		if len(vols) > 0 {
			args := append([]string{"volume", "rm", "-f"}, vols...)
			exec.Command("docker", args...).Run()
		}
	}
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func apiPost(path string, body interface{}) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, serverURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	return httpClient.Do(req)
}

func apiGet(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, serverURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	return httpClient.Do(req)
}

func apiDelete(path string) {
	req, err := http.NewRequest(http.MethodDelete, serverURL+path, nil)
	if err != nil {
		return
	}
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	if resp, err := httpClient.Do(req); err == nil {
		resp.Body.Close()
	}
}

func decodeJSON(t *testing.T, r io.Reader, v interface{}) {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("decode JSON: %v\nbody: %s", err, b)
	}
}

// ── Docker helpers ────────────────────────────────────────────────────────────

// freePort returns a local TCP port that is not currently bound.
func freePort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic("freePort: " + err.Error())
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// waitForKafkaBroker polls docker logs until the broker reports
// "Kafka Server started" or the timeout elapses.
func waitForKafkaBroker(t *testing.T, containerName string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("docker", "logs", containerName).CombinedOutput()
		if err == nil && strings.Contains(string(out), "Kafka Server started") {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("kafka broker %q did not start within %v", containerName, timeout)
}

// waitForContainer polls docker inspect until the container's State.Status
// equals "running" or the timeout elapses.
func waitForContainer(t *testing.T, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command(
			"docker", "inspect", "--format={{.State.Status}}", name,
		).Output()
		if err == nil && strings.TrimSpace(string(out)) == "running" {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("container %q did not reach status 'running' within %v", name, timeout)
}

// containerStatus returns the State.Status of a container, or "" on error.
func containerStatus(name string) string {
	out, err := exec.Command(
		"docker", "inspect", "--format={{.State.Status}}", name,
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// dockerExec runs a command inside a container and returns combined output.
func dockerExec(container string, args ...string) (string, error) {
	return dockerExecTimeout(30*time.Second, container, args...)
}

// dockerExecTimeout runs a command inside a container with a per-call deadline.
func dockerExecTimeout(timeout time.Duration, container string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmdArgs := append([]string{"exec", container}, args...)
	out, err := exec.CommandContext(ctx, "docker", cmdArgs...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// removeVolume removes a docker volume (best-effort, ignores errors).
func removeVolume(name string) {
	exec.Command("docker", "volume", "rm", "-f", name).Run()
}
