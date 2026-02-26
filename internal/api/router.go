package api

import (
	"net/http"
	"strings"

	"github.com/gsarma/localisprod-v2/internal/api/handlers"
	"github.com/gsarma/localisprod-v2/internal/auth"
	"github.com/gsarma/localisprod-v2/internal/store"
)

func NewRouter(s *store.Store, oauthSvc *auth.OAuthService, jwtSvc *auth.JWTService, appURL, rootEmail string) http.Handler {
	nodeH := handlers.NewNodeHandler(s)
	appH := handlers.NewApplicationHandler(s)
	depH := handlers.NewDeploymentHandler(s)
	dbH := handlers.NewDatabaseHandler(s)
	cacheH := handlers.NewCacheHandler(s)
	kafkaH := handlers.NewKafkaHandler(s)
	monitoringH := handlers.NewMonitoringHandler(s)
	objectStorageH := handlers.NewObjectStorageHandler(s)
	dashH := handlers.NewDashboardHandler(s)
	settingsH := handlers.NewSettingsHandler(s, appURL)
	githubH := handlers.NewGithubHandler(s)
	webhookH := handlers.NewWebhookHandler(s)
	authH := handlers.NewAuthHandler(s, oauthSvc, jwtSvc, appURL, rootEmail)
	providersH := handlers.NewProvidersHandler(s)

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
		case http.MethodPut:
			appH.Update(w, r, id)
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

	// Caches
	protectedMux.HandleFunc("/api/caches", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cacheH.List(w, r)
		case http.MethodPost:
			cacheH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	protectedMux.HandleFunc("/api/caches/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/caches/")
		id := strings.TrimSuffix(path, "/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			cacheH.Get(w, r, id)
		case http.MethodDelete:
			cacheH.Delete(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Kafkas
	protectedMux.HandleFunc("/api/kafkas", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			kafkaH.List(w, r)
		case http.MethodPost:
			kafkaH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	protectedMux.HandleFunc("/api/kafkas/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/kafkas/")
		id := strings.TrimSuffix(path, "/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			kafkaH.Get(w, r, id)
		case http.MethodDelete:
			kafkaH.Delete(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Monitorings
	protectedMux.HandleFunc("/api/monitorings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			monitoringH.List(w, r)
		case http.MethodPost:
			monitoringH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	protectedMux.HandleFunc("/api/monitorings/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/monitorings/")
		id := strings.TrimSuffix(path, "/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			monitoringH.Get(w, r, id)
		case http.MethodDelete:
			monitoringH.Delete(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Object Storages
	protectedMux.HandleFunc("/api/object-storages", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			objectStorageH.List(w, r)
		case http.MethodPost:
			objectStorageH.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	protectedMux.HandleFunc("/api/object-storages/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/object-storages/")
		id := strings.TrimSuffix(path, "/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			objectStorageH.Get(w, r, id)
		case http.MethodDelete:
			objectStorageH.Delete(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Cloud Providers
	protectedMux.HandleFunc("/api/providers/do/metadata", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			providersH.DOMetadata(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	protectedMux.HandleFunc("/api/providers/do/provision", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			providersH.DOProvision(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	protectedMux.HandleFunc("/api/providers/aws/metadata", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			providersH.AWSMetadata(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	protectedMux.HandleFunc("/api/providers/aws/provision", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			providersH.AWSProvision(w, r)
		} else {
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

	return securityHeadersMiddleware(corsMiddleware(bodyLimitMiddleware(mainMux), appURL))
}

// bodyLimitMiddleware caps request bodies at 1 MiB to prevent DoS via large payloads.
func bodyLimitMiddleware(next http.Handler) http.Handler {
	const maxBytes = 1 << 20 // 1 MiB
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
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

// securityHeadersMiddleware adds common security-related HTTP response headers.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:;")
		next.ServeHTTP(w, r)
	})
}
