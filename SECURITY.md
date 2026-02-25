# Security Analysis

**Date:** 2026-02-25
**Scope:** Full codebase — Go backend, React/TypeScript frontend

---

## API Protection Status

All `/api/` routes require a valid JWT session cookie enforced by `jwtSvc.Middleware()` in `internal/api/router.go:377`. Intentional public exceptions:

| Route | Reason |
|---|---|
| `GET /api/auth/google` | OAuth login initiation |
| `GET /api/auth/google/callback` | OAuth callback |
| `POST /api/webhooks/github/:token` | GitHub webhook (token-authenticated) |

---

## Findings

### CRITICAL

#### 1. Command Injection via `app.Command`
- **File:** `internal/sshexec/executor.go:186-188`
- **Description:** Every field in `DockerRunCmd()` is passed through `shellEscape()` except `cfg.Command`, which is appended directly to the shell command string. Any authenticated user can create an application with `"command": "; rm -rf /"` and get arbitrary shell execution on the target node (or on the management node itself via `LocalRunner`).
- **Affected:** All users (not just root)
- **Fix:** Wrap `cfg.Command` in `shellEscape()`, or better, split it into a `[]string` slice and pass each argument individually.

---

### HIGH

#### 2. Session Cookie Missing `Secure` Flag
- **File:** `internal/api/handlers/auth.go:81-88`
- **Description:** The `session` JWT cookie is issued without `Secure: true`. This allows the cookie to be transmitted over plaintext HTTP, making it susceptible to interception.
- **Fix:** Set `Secure: true` on the cookie. When running in production over HTTPS this flag must be present.

#### 3. Webhook HMAC Signature Verification is Optional
- **File:** `internal/api/handlers/webhook.go:44-51`
- **Description:** HMAC-SHA256 signature verification only runs if the user has configured `webhook_secret`. If not set, any POST to `/api/webhooks/github/:token` triggers redeploys — the URL token is the sole protection. Tokens are short (UUID-derived) and the endpoint is unauthenticated.
- **Fix:** Either require `webhook_secret` to be set before the webhook endpoint is active, or return `401` when no secret is configured.

#### 4. Unbounded Request Bodies
- **File:** `internal/api/handlers/webhook.go:26`; all handlers via `json.NewDecoder(r.Body)`
- **Description:** `io.ReadAll(r.Body)` and `json.NewDecoder(r.Body).Decode()` have no size limit. A large payload will be fully buffered in memory, enabling denial-of-service.
- **Fix:** Wrap `r.Body` with `http.MaxBytesReader(w, r.Body, maxBytes)` at the start of each handler or in middleware.

#### 5. Internal Errors Leaked to Clients
- **File:** `internal/api/handlers/nodes.go:70,84,109`; `deployments.go:98`; `applications.go` and others
- **Description:** `writeError(w, http.StatusInternalServerError, err.Error())` returns raw database and SSH error messages to the client. These can expose schema details, file paths, and SSH key parsing errors.
- **Fix:** Log the full error server-side (`log.Printf`) and return a generic message to the client (e.g. `"internal server error"`).

#### 6. SSH Host Key Verification Disabled
- **File:** `internal/sshexec/executor.go:70`
- **Description:** `ssh.InsecureIgnoreHostKey()` disables host key verification, making all SSH connections vulnerable to man-in-the-middle attacks. An attacker on the network path can impersonate any registered node and receive arbitrary commands.
- **Note:** This is a known design tradeoff for a cluster management tool. For production use, host keys should be stored per node and verified on each connection.

---

### MEDIUM

#### 7. No HTTP Security Headers
- **File:** `internal/api/router.go:389-401` (CORS middleware)
- **Description:** The application sets no security-related HTTP response headers. Missing headers:
  - `X-Frame-Options: DENY` — clickjacking protection
  - `X-Content-Type-Options: nosniff` — MIME sniffing protection
  - `Content-Security-Policy` — XSS mitigation for the frontend
  - `Referrer-Policy: strict-origin-when-cross-origin`
- **Fix:** Add a security headers middleware applied to all responses.

#### 8. No Rate Limiting
- **File:** `internal/api/router.go` — all routes
- **Description:** No rate limiting is applied to any endpoint. High-risk targets:
  - `/api/webhooks/github/:token` — brute-force token enumeration
  - `/api/nodes/:id/ping` and deploy/restart endpoints — each call initiates an SSH connection and may trigger Docker operations
- **Fix:** Add per-IP rate limiting middleware (e.g. `golang.org/x/time/rate`).

#### 9. JWT Tokens: 30-Day Expiry, No Revocation
- **File:** `internal/auth/jwt.go:35`
- **Description:** Tokens expire after 30 days with no server-side revocation. If `ROOT_EMAIL` is changed, the old root user's token retains root privileges until expiry. A compromised token cannot be invalidated without rotating `JWT_SECRET` (which invalidates all sessions).
- **Fix:** Implement a token blocklist in the store, or shorten the expiry and add refresh tokens.

#### 10. OAuth Error Details Returned to Client
- **File:** `internal/api/handlers/auth.go:64,70`
- **Description:** `"oauth exchange failed: " + err.Error()` and `"failed to upsert user: " + err.Error()` expose internal OAuth provider errors and database error messages to the client during login.
- **Fix:** Log the detailed error, return a generic `"authentication failed"` message.

---

### LOW

#### 11. Port Range Not Validated
- **File:** `internal/api/handlers/deployments.go:69`
- **Description:** Port numbers parsed from application config are not validated against the valid range (1–65535). Negative values or ports above 65535 are accepted and passed to Docker.
- **Fix:** Add an explicit range check after parsing.

#### 12. Container Name Sanitization Incomplete
- **File:** `internal/api/handlers/deployments.go:86`
- **Description:** `strings.ReplaceAll(app.Name, " ", "-")` only strips spaces. Application names containing characters like `/`, `;`, or `&` produce unusual container names. These are eventually `shellEscape`d before SSH use, but the container name itself may confuse Docker.
- **Fix:** Restrict application names to `[a-zA-Z0-9_-]` at creation time.

#### 13. Open Redirect via Node Host in Frontend URLs
- **File:** `web/src/pages/Monitorings.tsx:108,118`
- **Description:** Monitoring page constructs `href` values directly from `m.node_host` and port fields returned by the API without validation: `` href={`http://${m.node_host}:${m.prometheus_port}`} ``. A malicious backend value could produce unexpected URLs.
- **Fix:** Validate that `node_host` is a valid hostname or IP before constructing links.

---

## Summary Table

| # | Severity | File | Issue |
|---|---|---|---|
| 1 | CRITICAL | `internal/sshexec/executor.go:186` | `app.Command` not shell-escaped — command injection |
| 2 | HIGH | `internal/api/handlers/auth.go:81` | Session cookie missing `Secure` flag |
| 3 | HIGH | `internal/api/handlers/webhook.go:44` | Webhook HMAC verification optional |
| 4 | HIGH | `internal/api/handlers/webhook.go:26` + all handlers | No request body size limit |
| 5 | HIGH | Multiple handlers | Raw `err.Error()` returned to clients |
| 6 | HIGH | `internal/sshexec/executor.go:70` | `InsecureIgnoreHostKey` — SSH MITM |
| 7 | MEDIUM | `internal/api/router.go:389` | No HTTP security headers |
| 8 | MEDIUM | `internal/api/router.go` | No rate limiting |
| 9 | MEDIUM | `internal/auth/jwt.go:35` | 30-day JWT, no revocation |
| 10 | MEDIUM | `internal/api/handlers/auth.go:64,70` | OAuth errors leaked to client |
| 11 | LOW | `internal/api/handlers/deployments.go:69` | Port range not validated |
| 12 | LOW | `internal/api/handlers/deployments.go:86` | Container name not fully sanitized |
| 13 | LOW | `web/src/pages/Monitorings.tsx:108,118` | Open redirect via node host URLs |
