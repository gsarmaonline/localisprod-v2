package api

import (
	"net/http"
	"strings"

	"github.com/gsarma/localisprod-v2/internal/api/handlers"
	"github.com/gsarma/localisprod-v2/internal/auth"
	"github.com/gsarma/localisprod-v2/internal/store"
)

func NewRouter(s *store.Store, oauthSvc *auth.OAuthService, jwtSvc *auth.JWTService, appURL string) http.Handler {
	nodeH := handlers.NewNodeHandler(s)
	appH := handlers.NewApplicationHandler(s)
	depH := handlers.NewDeploymentHandler(s)
	dbH := handlers.NewDatabaseHandler(s)
	dashH := handlers.NewDashboardHandler(s)
	settingsH := handlers.NewSettingsHandler(s, appURL)
	githubH := handlers.NewGithubHandler(s)
	webhookH := handlers.NewWebhookHandler(s)
	authH := handlers.NewAuthHandler(s, oauthSvc, jwtSvc, appURL)

	// Unprotected mux (auth + webhooks)
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/api/auth/google", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authH.GoogleLogin(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	publicMux.HandleFunc("/api/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authH.GoogleCallback(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	// Per-user webhook: /api/webhooks/github/{token}
	publicMux.HandleFunc("/api/webhooks/github/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		token := strings.TrimPrefix(r.URL.Path, "/api/webhooks/github/")
		token = strings.TrimSuffix(token, "/")
		if token == "" {
			http.NotFound(w, r)
			return
		}
		webhookH.GithubForUser(w, r, token)
	})

	// Protected mux (all other API routes require JWT)
	protectedMux := http.NewServeMux()

	protectedMux.HandleFunc("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authH.Logout(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	protectedMux.HandleFunc("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authH.Me(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	protectedMux.HandleFunc("/api/stats", dashH.Stats)

	// Settings
	protectedMux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
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
	protectedMux.HandleFunc("/api/github/repos", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			githubH.ListRepos(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Nodes
	protectedMux.HandleFunc("/api/nodes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			nodeH.List(w, r)
		case http.MethodPost:
			nodeH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	protectedMux.HandleFunc("/api/nodes/", func(w http.ResponseWriter, r *http.Request) {
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
	protectedMux.HandleFunc("/api/applications", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			appH.List(w, r)
		case http.MethodPost:
			appH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	protectedMux.HandleFunc("/api/applications/", func(w http.ResponseWriter, r *http.Request) {
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

	// Databases
	protectedMux.HandleFunc("/api/databases", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			dbH.List(w, r)
		case http.MethodPost:
			dbH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	protectedMux.HandleFunc("/api/databases/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/databases/")
		id := strings.TrimSuffix(path, "/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			dbH.Get(w, r, id)
		case http.MethodDelete:
			dbH.Delete(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Deployments
	protectedMux.HandleFunc("/api/deployments", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			depH.List(w, r)
		case http.MethodPost:
			depH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	protectedMux.HandleFunc("/api/deployments/", func(w http.ResponseWriter, r *http.Request) {
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

	// Wrap protected routes with JWT middleware
	protectedHandler := jwtSvc.Middleware(protectedMux)

	// Main mux: public routes first, then protected
	mainMux := http.NewServeMux()
	mainMux.Handle("/api/auth/google", publicMux)
	mainMux.Handle("/api/auth/google/callback", publicMux)
	mainMux.Handle("/api/webhooks/github/", publicMux)
	mainMux.Handle("/api/", protectedHandler)

	return corsMiddleware(mainMux, appURL)
}

func corsMiddleware(next http.Handler, appURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", appURL)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
