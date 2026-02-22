package api

import (
	"net/http"
	"strings"

	"github.com/gsarma/localisprod-v2/internal/api/handlers"
	"github.com/gsarma/localisprod-v2/internal/store"
)

func NewRouter(s *store.Store) http.Handler {
	nodeH := handlers.NewNodeHandler(s)
	appH := handlers.NewApplicationHandler(s)
	depH := handlers.NewDeploymentHandler(s)
	dashH := handlers.NewDashboardHandler(s)
	settingsH := handlers.NewSettingsHandler(s)
	githubH := handlers.NewGithubHandler(s)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/stats", dashH.Stats)

	// Settings
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			settingsH.Get(w, r)
		case http.MethodPut:
			settingsH.Update(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// GitHub
	mux.HandleFunc("/api/github/repos", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			githubH.ListRepos(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Nodes
	mux.HandleFunc("/api/nodes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			nodeH.List(w, r)
		case http.MethodPost:
			nodeH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/nodes/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/nodes/")
		parts := strings.SplitN(path, "/", 2)
		id := parts[0]
		if id == "" {
			http.NotFound(w, r)
			return
		}

		if len(parts) == 2 {
			switch parts[1] {
			case "ping":
				if r.Method == http.MethodPost {
					nodeH.Ping(w, r, id)
				} else {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				}
				return
			case "setup-traefik":
				if r.Method == http.MethodPost {
					nodeH.SetupTraefik(w, r, id)
				} else {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}
		}

		switch r.Method {
		case http.MethodGet:
			nodeH.Get(w, r, id)
		case http.MethodDelete:
			nodeH.Delete(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Applications
	mux.HandleFunc("/api/applications", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			appH.List(w, r)
		case http.MethodPost:
			appH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/applications/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/applications/")
		id := strings.TrimSuffix(path, "/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			appH.Get(w, r, id)
		case http.MethodDelete:
			appH.Delete(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Deployments
	mux.HandleFunc("/api/deployments", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			depH.List(w, r)
		case http.MethodPost:
			depH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/deployments/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/deployments/")
		parts := strings.SplitN(path, "/", 2)
		id := parts[0]
		if id == "" {
			http.NotFound(w, r)
			return
		}

		if len(parts) == 2 {
			switch parts[1] {
			case "restart":
				if r.Method == http.MethodPost {
					depH.Restart(w, r, id)
				} else {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				}
				return
			case "logs":
				if r.Method == http.MethodGet {
					depH.Logs(w, r, id)
				} else {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}
		}

		switch r.Method {
		case http.MethodGet:
			depH.Get(w, r, id)
		case http.MethodDelete:
			depH.Delete(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
