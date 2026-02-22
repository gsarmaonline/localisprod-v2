.PHONY: dev dev-backend dev-frontend build build-frontend build-backend run clean

# Run both backend and frontend dev servers concurrently
dev:
	@(trap 'kill 0' INT; air & cd web && npm run dev)

# Run Go backend with auto-reload (requires: go install github.com/air-verse/air@latest)
dev-backend:
	air

# Run Vite frontend dev server (proxies /api to :8080)
dev-frontend:
	cd web && npm run dev

# Build frontend then backend binary
build: build-frontend build-backend

# Build only the React frontend into web/dist
build-frontend:
	cd web && npm run build

# Build only the Go binary
build-backend:
	go build -o bin/server ./cmd/server/main.go

# Build everything and run the server
run: build
	./bin/server

clean:
	rm -rf bin/ web/dist/
