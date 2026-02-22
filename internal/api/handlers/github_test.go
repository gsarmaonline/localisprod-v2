package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gsarma/localisprod-v2/internal/api/handlers"
)

func TestGithubListRepos_NoToken(t *testing.T) {
	s := newTestStore(t)
	h := handlers.NewGithubHandler(s)

	rec := httptest.NewRecorder()
	h.ListRepos(rec, getRequest("/api/github/repos"))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when no token, got %d (body: %s)", rec.Code, rec.Body)
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["error"] == "" {
		t.Error("expected error message in response")
	}
}

// rewriteTransport redirects all requests to a test server base URL.
// It holds a reference to the underlying real transport to avoid self-recursion.
type rewriteTransport struct {
	host      string // e.g. "127.0.0.1:12345"
	transport http.RoundTripper
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = rt.host
	return rt.transport.RoundTrip(req2)
}

// withFakeGitHub starts a mock HTTP server and replaces http.DefaultTransport
// for the duration of the test, forwarding all requests to the fake server.
func withFakeGitHub(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	realTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{
		host:      srv.Listener.Addr().String(),
		transport: realTransport,
	}
	t.Cleanup(func() { http.DefaultTransport = realTransport })
}

func TestGithubListRepos_Success(t *testing.T) {
	fakeRepos := []map[string]any{
		{
			"name":        "my-repo",
			"full_name":   "user/my-repo",
			"description": "A test repo",
			"private":     false,
			"html_url":    "https://github.com/user/my-repo",
		},
		{
			"name":        "private-repo",
			"full_name":   "user/private-repo",
			"description": "",
			"private":     true,
			"html_url":    "https://github.com/user/private-repo",
		},
	}
	withFakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeRepos)
	})

	s := newTestStore(t)
	_ = s.SetUserSetting(testUserID, "github_token", "ghp_testtoken")
	h := handlers.NewGithubHandler(s)

	rec := httptest.NewRecorder()
	h.ListRepos(rec, getRequest("/api/github/repos"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body)
	}

	var repos []map[string]any
	decodeJSON(t, rec, &repos)

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0]["name"] != "my-repo" {
		t.Errorf("expected name 'my-repo', got %v", repos[0]["name"])
	}
	if repos[1]["private"] != true {
		t.Errorf("expected private=true for second repo, got %v", repos[1]["private"])
	}
}

func TestGithubListRepos_APIError(t *testing.T) {
	withFakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	s := newTestStore(t)
	_ = s.SetUserSetting(testUserID, "github_token", "bad-token")
	h := handlers.NewGithubHandler(s)

	rec := httptest.NewRecorder()
	h.ListRepos(rec, getRequest("/api/github/repos"))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 when GitHub returns 401, got %d", rec.Code)
	}
}

func TestGithubListRepos_InvalidJSON(t *testing.T) {
	withFakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	})

	s := newTestStore(t)
	_ = s.SetUserSetting(testUserID, "github_token", "sometoken")
	h := handlers.NewGithubHandler(s)

	rec := httptest.NewRecorder()
	h.ListRepos(rec, getRequest("/api/github/repos"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for invalid JSON from GitHub, got %d", rec.Code)
	}
}
