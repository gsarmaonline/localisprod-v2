# Localisprod v2 — Cluster Manager

A cluster management system for deploying Docker containers to SSH-accessible nodes.

## Features

- **Google OAuth login** — each Google account gets an isolated workspace
- Register SSH nodes (host, port, username, private key)
- Define applications (Docker image, env vars, port mappings, command, linked databases)
- Provision managed databases (Postgres, MySQL, Redis, MongoDB) on nodes — connection URLs auto-injected into linked app deployments
- Deploy applications as Docker containers onto nodes via SSH
- View container logs, restart or stop deployments
- Dashboard with live counts across nodes, apps, and deployment statuses
- **GitHub webhook auto-redeploy**: automatically re-pulls and restarts containers when a new image is published to GHCR
- **Per-user webhook URL**: each user has a personal webhook endpoint so multiple accounts can integrate with different GitHub repos

## Tech Stack

- **Backend**: Go (`net/http`, `golang.org/x/crypto/ssh`, `modernc.org/sqlite`, `golang.org/x/oauth2`, `github.com/golang-jwt/jwt/v5`)
- **Frontend**: React 18 + TypeScript + Vite + Tailwind CSS
- **Database**: SQLite (`cluster.db`)
- **Auth**: Google OAuth 2.0 + JWT sessions (httpOnly cookie)

## Getting Started

### Prerequisites

- Go 1.21+
- Node.js 18+
- A Google Cloud project with an OAuth 2.0 Web Client credential

### Google OAuth Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/) → **APIs & Services** → **Credentials**
2. Create an **OAuth 2.0 Client ID** (Web application)
3. Add `{APP_URL}/api/auth/google/callback` as an **Authorized redirect URI**
4. Copy the **Client ID** and **Client Secret**

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

### Environment Variables

Can be set in a `.env` file at the project root or as real environment variables:

| Variable               | Default                        | Description |
|------------------------|--------------------------------|-------------|
| `JWT_SECRET`           | *(required)*                   | Secret for signing JWTs. Generate: `openssl rand -base64 32` |
| `GOOGLE_CLIENT_ID`     | *(required for login)*         | Google OAuth 2.0 client ID |
| `GOOGLE_CLIENT_SECRET` | *(required for login)*         | Google OAuth 2.0 client secret |
| `APP_URL`              | `http://localhost:8080`        | Public base URL (must match OAuth redirect URI) |
| `PORT`                 | `8080`                         | HTTP server port |
| `DB_PATH`              | `cluster.db`                   | Path to SQLite database |
| `SECRET_KEY`           | *(unset)*                      | Base64-encoded 32-byte key for AES-256-GCM encryption of env vars. Generate: `openssl rand -base64 32` |

`.env` example:
```
JWT_SECRET="<openssl rand -base64 32>"
GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com"
GOOGLE_CLIENT_SECRET="GOCSPX-your-secret"
APP_URL="http://localhost:8080"
SECRET_KEY="<openssl rand -base64 32>"
```

## API

All `/api/*` routes except `/api/auth/google`, `/api/auth/google/callback`, and `/api/webhooks/github/{token}` require a valid session cookie (set after Google login).

| Method | Path                                  | Description                      |
|--------|---------------------------------------|----------------------------------|
| GET    | `/api/auth/google`                    | Start Google OAuth flow          |
| GET    | `/api/auth/google/callback`           | OAuth callback                   |
| GET    | `/api/auth/me`                        | Current user info                |
| POST   | `/api/auth/logout`                    | Clear session                    |
| POST   | `/api/nodes`                          | Register a node                  |
| GET    | `/api/nodes`                          | List nodes                       |
| DELETE | `/api/nodes/:id`                      | Delete node                      |
| POST   | `/api/nodes/:id/ping`                 | Test SSH connectivity            |
| POST   | `/api/applications`                   | Create application               |
| GET    | `/api/applications`                   | List applications                |
| DELETE | `/api/applications/:id`               | Delete application               |
| POST   | `/api/databases`                      | Provision a database             |
| GET    | `/api/databases`                      | List databases                   |
| GET    | `/api/databases/:id`                  | Get database                     |
| DELETE | `/api/databases/:id`                  | Stop + remove database           |
| POST   | `/api/deployments`                    | Deploy app to node               |
| GET    | `/api/deployments`                    | List deployments                 |
| DELETE | `/api/deployments/:id`                | Stop + remove deployment         |
| POST   | `/api/deployments/:id/restart`        | Restart container                |
| GET    | `/api/deployments/:id/logs`           | Fetch last 200 log lines         |
| GET    | `/api/stats`                          | Dashboard counts                 |
| GET    | `/api/settings`                       | Get GitHub + webhook settings    |
| PUT    | `/api/settings`                       | Update GitHub + webhook settings |
| POST   | `/api/webhooks/github/{token}`        | Per-user GitHub registry webhook |

## GitHub Webhook Auto-Redeploy

Each user gets a personal webhook URL (shown in **Settings**) of the form `{APP_URL}/api/webhooks/github/{webhook_token}`.

When an application has a `github_repo` set, publishing a new image to GHCR automatically redeploys all running deployments for that app.

1. In **Settings**, enter a **Webhook Secret** and copy the **Webhook URL**.
2. In your GitHub repository → **Settings** → **Webhooks** → **Add webhook**:
   - Payload URL: the Webhook URL from step 1
   - Content type: `application/json`
   - Secret: the same value as step 1
   - Events: choose **Registry packages**
3. On each new image publish, the server will `docker pull`, stop/remove the old container, and start a fresh one with the same config.
