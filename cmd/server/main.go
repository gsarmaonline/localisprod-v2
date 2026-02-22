package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gsarma/localisprod-v2/internal/api"
	"github.com/gsarma/localisprod-v2/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "cluster.db"
	}

	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}

	router := api.NewRouter(s)

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
