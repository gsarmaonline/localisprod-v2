package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gsarma/localisprod-v2/internal/api"
	"github.com/gsarma/localisprod-v2/internal/auth"
	"github.com/gsarma/localisprod-v2/internal/secret"
	"github.com/gsarma/localisprod-v2/internal/store"
)

func main() {
	loadDotEnv(".env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "cluster.db"
	}

	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = fmt.Sprintf("http://localhost:%s", port)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required (generate with: openssl rand -base64 32)")
	}

	var cipher *secret.Cipher
	if keyB64 := os.Getenv("SECRET_KEY"); keyB64 != "" {
		keyBytes, err := base64.StdEncoding.DecodeString(keyB64)
		if err != nil || len(keyBytes) != 32 {
			log.Fatalf("SECRET_KEY must be a base64-encoded 32-byte key (generate with: openssl rand -base64 32)")
		}
		cipher, err = secret.New(keyBytes)
		if err != nil {
			log.Fatalf("failed to init cipher: %v", err)
		}
		log.Println("encryption enabled: env vars will be encrypted at rest")
	} else {
		log.Println("WARNING: SECRET_KEY not set — env vars stored in plaintext. Set SECRET_KEY to enable encryption.")
	}

	s, err := store.New(dbPath, cipher)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}

	jwtSvc := auth.NewJWTService(jwtSecret)

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	oauthSvc := auth.NewOAuthService(googleClientID, googleClientSecret, appURL)

	router := api.NewRouter(s, oauthSvc, jwtSvc, appURL)

	mux := http.NewServeMux()

	// API routes
	mux.Handle("/api/", router)

	// Static frontend — serve web/dist if it exists
	staticDir := "web/dist"
	if _, err := os.Stat(staticDir); err == nil {
		fs := http.FileServer(http.Dir(staticDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback: if the file doesn't exist, serve index.html
			path := staticDir + r.URL.Path
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, staticDir+"/index.html")
				return
			}
			fs.ServeHTTP(w, r)
		})
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "Cluster Management API — build the frontend with: make build-frontend")
		})
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// loadDotEnv reads a .env file and sets any variables not already present
// in the environment. Supports both `KEY=VALUE` and `export KEY="VALUE"` syntax.
// Missing or unreadable files are silently ignored.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip leading "export " if present
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// Strip surrounding quotes
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		// Only set if not already in the environment
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}
