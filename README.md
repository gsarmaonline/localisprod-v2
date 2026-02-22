# Localisprod v2 — Cluster Manager

A cluster management system for deploying Docker containers to SSH-accessible nodes.

## Features

- Register SSH nodes (host, port, username, private key)
- Define applications (Docker image, env vars, port mappings, command)
- Deploy applications as Docker containers onto nodes via SSH
- View container logs, restart or stop deployments
- Dashboard with live counts across nodes, apps, and deployment statuses
- **GitHub webhook auto-redeploy**: automatically re-pulls and restarts containers when a new image is published to GHCR

## Tech Stack

- **Backend**: Go (`net/http`, `golang.org/x/crypto/ssh`, `modernc.org/sqlite`)
- **Frontend**: React 18 + TypeScript + Vite + Tailwind CSS
- **Database**: SQLite (`cluster.db`)

## Getting Started

### Prerequisites

- Go 1.21+
- Node.js 18+

### Development

```bash
# Terminal 1 — Go API server on :8080
make dev-backend

# Terminal 2 — Vite dev server on :5173 (proxies /api → :8080)
make dev-frontend
```

Open http://localhost:5173

### Production

```bash
# Build frontend + backend binary
make build

# Run the server (serves API + static frontend on :8080)
make run
```

Or run the binary directly:

```bash
./bin/server
```

Environment variables (can also be set in a `.env` file at the project root):

| Variable     | Default      | Description                                                        |
|--------------|--------------|--------------------------------------------------------------------|
| `PORT`       | `8080`       | HTTP server port                                                   |
| `DB_PATH`    | `cluster.db` | Path to SQLite database                                            |
| `SECRET_KEY` | *(unset)*    | Base64-encoded 32-byte key for AES-256-GCM encryption of env vars. Generate with `openssl rand -base64 32`. Without this, env vars are stored in plaintext. |

`.env` example:
```
SECRET_KEY="<output of openssl rand -base64 32>"
```

## API

| Method | Path                              | Description                    |
|--------|-----------------------------------|--------------------------------|
| POST   | `/api/nodes`                      | Register a node                |
| GET    | `/api/nodes`                      | List nodes                     |
| DELETE | `/api/nodes/:id`                  | Delete node                    |
| POST   | `/api/nodes/:id/ping`             | Test SSH connectivity          |
| POST   | `/api/applications`               | Create application             |
| GET    | `/api/applications`               | List applications               |
| DELETE | `/api/applications/:id`           | Delete application             |
| POST   | `/api/deployments`                | Deploy app to node             |
| GET    | `/api/deployments`                | List deployments               |
| DELETE | `/api/deployments/:id`            | Stop + remove deployment       |
| POST   | `/api/deployments/:id/restart`    | Restart container              |
| GET    | `/api/deployments/:id/logs`       | Fetch last 200 log lines       |
| GET    | `/api/stats`                      | Dashboard counts               |
| GET    | `/api/settings`                   | Get GitHub + webhook settings  |
| PUT    | `/api/settings`                   | Update GitHub + webhook settings |
| POST   | `/api/webhooks/github`            | GitHub registry_package webhook |

## GitHub Webhook Auto-Redeploy

When an application has a `github_repo` set, you can configure a GitHub webhook so that publishing a new image to GHCR automatically redeploys all running deployments for that app.

1. In **Settings**, enter a **Webhook Secret** and note the **Webhook URL** (`/api/webhooks/github`).
2. In your GitHub repository → **Settings** → **Webhooks** → **Add webhook**:
   - Payload URL: the Webhook URL from step 1
   - Content type: `application/json`
   - Secret: the same value as step 1
   - Events: choose **Registry packages**
3. On each new image publish, the server will `docker pull`, stop/remove the old container, and start a fresh one with the same config.
