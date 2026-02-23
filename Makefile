DEPLOY_HOST ?= root@localisprod.com
DEPLOY_KEY  ?= ~/.ssh/digitalocean
DEPLOY_DIR  ?= /opt/localisprod/app

.PHONY: dev dev-backend dev-frontend build build-frontend build-backend run clean deploy deploy-backend deploy-frontend

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

# Build Linux binary for deployment
build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/server-linux ./cmd/server/main.go

# Deploy backend binary only (cross-compile + stop service + upload + restart)
deploy-backend: build-linux
	ssh -i $(DEPLOY_KEY) $(DEPLOY_HOST) "systemctl stop localisprod"
	scp -i $(DEPLOY_KEY) bin/server-linux $(DEPLOY_HOST):$(DEPLOY_DIR)/bin/server
	ssh -i $(DEPLOY_KEY) $(DEPLOY_HOST) "chmod +x $(DEPLOY_DIR)/bin/server && chown localisprod:localisprod $(DEPLOY_DIR)/bin/server && systemctl start localisprod"

# Deploy frontend assets only
deploy-frontend: build-frontend
	rsync -az --delete -e "ssh -i $(DEPLOY_KEY)" web/dist/ $(DEPLOY_HOST):$(DEPLOY_DIR)/web/dist/

# Full deploy: frontend + backend + service files
deploy: build-frontend build-linux
	rsync -az --delete -e "ssh -i $(DEPLOY_KEY)" web/dist/ $(DEPLOY_HOST):$(DEPLOY_DIR)/web/dist/
	ssh -i $(DEPLOY_KEY) $(DEPLOY_HOST) "systemctl stop localisprod"
	scp -i $(DEPLOY_KEY) bin/server-linux $(DEPLOY_HOST):$(DEPLOY_DIR)/bin/server
	ssh -i $(DEPLOY_KEY) $(DEPLOY_HOST) "chmod +x $(DEPLOY_DIR)/bin/server && chown localisprod:localisprod $(DEPLOY_DIR)/bin/server && systemctl start localisprod"
	@echo "Deploy complete. Service status:"
	@ssh -i $(DEPLOY_KEY) $(DEPLOY_HOST) "systemctl status localisprod --no-pager -l"
