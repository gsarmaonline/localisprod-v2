package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gsarma/localisprod-v2/internal/auth"
	"github.com/gsarma/localisprod-v2/internal/models"
	"github.com/gsarma/localisprod-v2/internal/store"
)

const testUserID = "test-user-id"

// newTestStore creates an in-memory SQLite store for handler tests.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:", nil)
	if err != nil {
		t.Fatalf("newTestStore: %v", err)
	}
	return s
}

// withUserID injects fake JWT claims into the request context so handlers
// can extract the user ID without a real JWT cookie.
func withUserID(r *http.Request) *http.Request {
	claims := &auth.Claims{UserID: testUserID, Email: "test@example.com", Name: "Test User"}
	ctx := auth.InjectClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// postJSON creates a POST request with a JSON body, with auth claims injected.
func postJSON(t *testing.T, path string, body any) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	return withUserID(r)
}

// getRequest creates a GET request with auth claims injected.
func getRequest(path string) *http.Request {
	return withUserID(httptest.NewRequest(http.MethodGet, path, nil))
}

// decodeJSON decodes the response body into v.
func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decodeJSON: %v (body: %s)", err, rec.Body.String())
	}
}

// mustCreateNode inserts a node into the store and returns it.
func mustCreateNode(t *testing.T, s *store.Store) *models.Node {
	t.Helper()
	n := &models.Node{
		ID:        "test-node-id",
		Name:      "test-node",
		Host:      "127.0.0.1",
		Port:      22,
		Username:  "root",
		PrivateKey: "fake-key",
		Status:    "unknown",
		IsLocal:   true, // local so SSH isn't needed in tests
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateNode(n, testUserID); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	return n
}

// mustCreateApp inserts an application into the store and returns it.
func mustCreateApp(t *testing.T, s *store.Store) *models.Application {
	t.Helper()
	a := &models.Application{
		ID:          "test-app-id",
		Name:        "test-app",
		DockerImage: "nginx:latest",
		EnvVars:     `{}`,
		Ports:       `[]`,
		Command:     "",
		GithubRepo:  "",
		Domain:      "",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateApplication(a, testUserID); err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}
	return a
}

// mustCreateDeployment inserts a deployment into the store and returns it.
func mustCreateDeployment(t *testing.T, s *store.Store, appID, nodeID string) *models.Deployment {
	t.Helper()
	d := &models.Deployment{
		ID:            "test-dep-id",
		ApplicationID: appID,
		NodeID:        nodeID,
		ContainerName: "localisprod-test-app-abcd1234",
		ContainerID:   "abc123",
		Status:        "running",
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.CreateDeployment(d, testUserID); err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	return d
}

