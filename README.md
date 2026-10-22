# W-dev Documentation (Mini PaaS)

W-dev is a mini deployment platform similar to a simplified Vercel/Heroku. Users log in with GitHub, select a repo (or upload a ZIP), and the app is automatically built with Cloud Native Buildpacks, run as a Docker container, and accessed via `https://<slug>.<MAIN_DOMAIN>`.

## Table of contents

1. [Architecture](#1-architecture)
2. [Project structure](#2-project-structure)
3. [Prerequisites & installation](#3-prerequisites--installation)
4. [Configuration (.env)](#4-configuration-env)
5. [GitHub OAuth setup](#5-github-oauth-setup)
6. [Cloudflare Tunnel & Traefik setup](#6-cloudflare-tunnel--traefik-setup)
7. [Authentication flow](#7-authentication-flow)
8. [Deploy pipeline](#8-deploy-pipeline)
9. [API reference](#9-api-reference)
10. [WebSocket (real-time logs)](#10-websocket-real-time-logs)
11. [Database schema](#11-database-schema)
12. [Security](#12-security)
13. [Testing](#13-testing)
14. [Troubleshooting](#14-troubleshooting)

---

## 1. Architecture

```
                    ┌────────────┐
 Browser ──HTTPS──► │ Cloudflare │ ──Tunnel──► cloudflared ──► Traefik (:80)
                    └────────────┘                               │  routing via Docker labels
                                                                 ├─► api.<domain>   → paas-backend (:8080)
                                                                 └─► <slug>.<domain> → paas-app-<slug>

 paas-backend ── Docker socket ──► build (pack) · app container · database container
      │
      └── PostgreSQL (metadata: apps, deployments, env_vars, user_tokens, ...)
```

All containers live on a single Docker bridge network named `proxy-net`, so an app can reach its database by hostname (`<slug>-db`) and Traefik can discover app containers via labels.

Main components:

| Component | Role |
|---|---|
| **Go Backend** | API, GitHub OAuth, deploy queue & worker, WebSocket |
| **Traefik** | Dynamic reverse proxy; reads container labels (`traefik.http.routers...`) |
| **cloudflared** | Connects Cloudflare to Traefik without opening ports |
| **pack (Buildpacks)** | Turns source code into an image without a Dockerfile |
| **PostgreSQL** | Platform metadata store |

## 2. Project structure

```
cmd/server/main.go          Entry point: config → DB → Docker → worker → HTTP, graceful shutdown
internal/
  config/                   Environment variable parsing
  constants/                Network name, statuses, paths, timeouts
  model/                    Request/response DTOs
  repository/               GORM models, repositories, SQL migrations
  service/
    github_service.go       OAuth, repo listing, webhooks, signature verification
    git_service.go          git clone (with secure token injection)
    upload_service.go       ZIP extraction (anti zip-slip)
    build_service.go        `pack build` wrapper + log capture
    container_service.go    Runs the app container
    traefik_service.go      Traefik labels & public URL
    database_service.go     Provision isolated DB (postgres/mysql/mongodb/redis)
    app_service.go          Orchestrates create/deploy/delete app
    cleanup_service.go      Cleans up all app resources
    token_service.go        Store/read encrypted GitHub token
  worker/                   Queue (queue.go) & deploy worker (deploy_worker.go)
  websocket/                Hub (room per slug) & client
  handler/                  auth, github, deploy, app, webhook, ws
  middleware/               auth (session token), CORS
  router/                   Route registration (net/http ServeMux)
pkg/
  docker/                   Docker client + EnsureNetwork
  cloudflared/              Cloudflare SDK (tunnel & DNS) — optional
  util/                     crypto AES-GCM, session HMAC, slug, password, logger
deployments/                Dockerfile, docker-compose.yml, traefik/traefik.yml
scripts/setup.sh            Environment preparation
```

## 3. Prerequisites & installation

Prerequisites: Docker (with access to `/var/run/docker.sock`), Go matching `go.mod`, `git`, and the [`pack`](https://buildpacks.io/docs/tools/pack/) CLI.

```bash
./scripts/setup.sh      # creates proxy-net, workspace folder, .env (with random secrets), installs pack
go mod tidy             # generates go.sum
go run ./cmd/server     # development mode
```

Full stack with Docker Compose:

```bash
docker compose -f deployments/docker-compose.yml --env-file .env up -d --build
```

Health check: `GET /healthz` → `{"status":"ok"}`.

## 4. Configuration (.env)

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Backend HTTP port |
| `APP_ENV` | `development` | `production` enables the `Secure` cookie and requires a `JWT_SECRET` other than the default |
| `MAIN_DOMAIN` | — (required) | Parent domain; apps are served at `<slug>.<MAIN_DOMAIN>` |
| `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` | — (required) | Cloudflare credentials |
| `CLOUDFLARE_ZONE_ID` | | Zone used for custom-domain CNAME records |
| `CLOUDFLARE_TUNNEL_ID` | | UUID of the Master Tunnel; custom domains CNAME to `<UUID>.cfargotunnel.com` |
| `WORKSPACE_PATH` | `/var/lib/paas/workspace` | Location for cloned/extracted source & uploads |
| `DB_HOST/PORT/USER/PASSWORD/NAME/SSLMODE` | `DB_PASSWORD` required | Platform PostgreSQL connection |
| `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` | | OAuth App credentials |
| `GITHUB_WEBHOOK_SECRET` | | Webhook HMAC secret. **Required**; empty = all webhooks rejected |
| `GITHUB_WEBHOOK_BASE_URL` | `https://api.<MAIN_DOMAIN>` | Public backend URL for webhooks |
| `GITHUB_REDIRECT_URL` | `<BASE_URL>/api/auth/github/callback` | OAuth callback |
| `FRONTEND_URL` | `http://localhost:5173` | Post-login redirect target |
| `JWT_SECRET` | `change-me` | Signs sessions and encrypts GitHub tokens |
| `SESSION_TTL_HOURS` | `168` | Session lifetime |
| `CORS_ORIGINS` | `*` | Allowed origins (comma-separated). Use specific origins in production |
| `WORKER_COUNT` / `QUEUE_SIZE` | `2` / `64` | Number of parallel workers / queue capacity |
| `BUILDER_IMAGE` | Paketo builder-jammy-base | Builder for `pack` |
| `TRAEFIK_TLS` | `false` | `false`: TLS terminated at Cloudflare (entrypoint `web`). `true`: Traefik + ACME (`websecure`) |
| `TRAEFIK_CERT_RESOLVER` | `cloudflare` | Traefik cert resolver name |

Generate a random secret: `openssl rand -hex 32`.

## 5. GitHub OAuth setup

1. Open <https://github.com/settings/developers> → **New OAuth App**.
2. **Authorization callback URL** = the value of `GITHUB_REDIRECT_URL`.
3. Copy the Client ID & Client Secret into `.env`.
4. Scopes requested by the app: `read:user repo admin:repo_hook` (read repos including private ones, and install webhooks).

The webhook is installed automatically on the first deploy of a repo when `auto_deploy` is enabled (default). The webhook points to `GITHUB_WEBHOOK_BASE_URL/api/webhook/github`, listens for the `push` event, and uses `GITHUB_WEBHOOK_SECRET`. If installation fails (e.g., the token lacks admin permission on the repo), the deploy still proceeds and the message is returned in `warnings` on the response.

## 6. Cloudflare Tunnel & Traefik setup

1. Create one **Master Tunnel** via CLI (`cloudflared tunnel create <name>`), put its UUID in `CLOUDFLARE_TUNNEL_ID` and in `deployments/cloudflare/config.yml`.
2. Route the wildcard once: `cloudflared tunnel route dns <name> "*.yourdomain.com"` (ingress in `config.yml` sends `*.yourdomain.com` and `api.yourdomain.com` to `http://traefik:80`).
3. `docker compose up -d` runs `cloudflared` alongside Traefik and the backend. Deployments never call the Cloudflare API; Traefik routes by Host label. Only external custom domains create a CNAME to the Master Tunnel.

Each app container is given Traefik labels automatically:

```
traefik.enable=true
traefik.http.routers.paas-<slug>.rule=Host(`<slug>.<MAIN_DOMAIN>`)
traefik.http.routers.paas-<slug>.entrypoints=web            # websecure if TRAEFIK_TLS=true
traefik.http.services.paas-<slug>.loadbalancer.server.port=<port>
```

Without Cloudflare Tunnel, set `TRAEFIK_TLS=true`; Traefik will request a certificate via ACME DNS-01 Cloudflare (see `deployments/traefik/traefik.yml`, fill in your email).

## 7. Authentication flow

1. The frontend redirects the user to `GET /api/auth/github`. The backend generates a random `state` (cookie `oauth_state`, 10 minutes) and then redirects to GitHub.
2. GitHub calls `GET /api/auth/github/callback?code=…&state=…`. The backend validates `state`, exchanges `code` for an access token, reads the `/user` profile, and stores the token **encrypted with AES-256-GCM**.
3. The backend creates an HMAC-SHA256-signed session token (`payload.signature`), sends it as an HttpOnly `paas_session` cookie **and** via a redirect to `FRONTEND_URL/auth/callback#token=<session>` for cross-origin frontends.
4. Subsequent requests send `Authorization: Bearer <session>` (or the cookie). WebSockets use `?token=<session>` because browsers cannot send custom headers.

`POST /api/auth/logout` clears the cookie. Add `?revoke=true` to also delete the stored GitHub token (webhooks/auto-deploys for that user's apps can no longer clone private repos until they log in again).

## 8. Deploy pipeline

`POST /api/deploy` only validates the input, creates/updates the app record, installs the webhook, creates a deployment record with status `pending`, then enqueues the job and immediately responds `202 Accepted`. The heavy work is done by an asynchronous worker:

| Step | Detail |
|---|---|
| 1. Source | GitHub: `git clone --depth 1 --branch <branch>` into `workspace/repos/<slug>` (token injected via temporary HTTP header). ZIP: extract with zip-slip protection; a single root folder is automatically flattened |
| 2. Database (optional) | If `database` is set and doesn't exist yet, provision a DB container + volume on `proxy-net`, save `DATABASE_URL` into the app's env vars |
| 3. Env | All app env vars are read; `PORT` is always set to match the app's `port`; variables prefixed with `BP_` are also passed as build-time env |
| 4. Build | `pack build paas/<slug>:<deployment_id>` using the builder from `BUILDER_IMAGE`; logs streamed line by line |
| 5. Run | Old container stopped & removed, new container created on `proxy-net` (1 vCPU, 1 GB, restart `unless-stopped`) with Traefik labels |
| 6. Health check | Attempts a TCP connection to `paas-app-<slug>:<port>` for up to 45 seconds. Warning only on failure (effective if the backend is on `proxy-net`) |
| 7. Finish | Old image removed, deployment status set to `running`, `done` event sent |

Deployment status: `pending → building → deploying → running | failed`.

Important behavior:
- **One deploy per app at a time** (per-app mutex); other deploys for the same app wait.
- If the **build fails** and the app was previously running, the old container stays alive and the app status remains `running` (`last_error` is populated). If the failure occurs while starting the new container, the app is marked `failed`.
- Queue full → `503` with the message "deploy queue is full".
- Redeploying with the same slug updates the existing app's configuration (only by its owner; other owners receive `409`).
- Slugs `api, app, www, admin, traefik, dashboard, mail, ftp, ns1, ns2` are reserved. Slugs are auto-generated from `name`/repo name, up to 40 characters.

## 9. API reference

All endpoints (except those marked public) require a session. Errors are always of the form `{"error": "message"}`.

### Auth

| Method | Path | Description |
|---|---|---|
| GET | `/api/auth/github` | Public. Redirects to GitHub |
| GET | `/api/auth/github/callback` | Public. Completes login |
| GET | `/api/auth/me` | `{login, name, avatar_url}` |
| POST | `/api/auth/logout[?revoke=true]` | Logout |

### GitHub

`GET /api/github/repos?page=1&per_page=30&type=all&sort=updated&direction=desc`

- `type`: `all|owner|public|private|member` · `sort`: `created|updated|pushed|full_name`
- Response: `{"repositories":[{id,name,full_name,private,clone_url,default_branch,description,language,updated_at,...}], "page":1, "per_page":30}`
- `401` if the GitHub token is missing/expired → ask the user to log in again.

### Deploy

`POST /api/deploy`

**From GitHub** (`Content-Type: application/json`):

```json
{
  "name": "My Blog",
  "slug": "blog",
  "repo_full_name": "octocat/hello-world",
  "branch": "main",
  "port": 3000,
  "buildpack": "",
  "auto_deploy": true,
  "database": "postgres",
  "env": { "NODE_ENV": "production" }
}
```

Only `repo_full_name` is required; `slug` is generated from `name`/repo name if empty. `database`: `postgres | mysql | mongodb | redis`.

**From ZIP** (`multipart/form-data`): `file` field (max 200 MB) + `name`, `slug`, `port`, `buildpack`, `database`, `env` (JSON string).

```bash
curl -X POST https://api.yourdomain.com/api/deploy \
  -H "Authorization: Bearer $TOKEN" \
  -F name="Landing" -F slug=landing -F port=3000 -F file=@app.zip
```

`202` response:

```json
{
  "app": { "id": 1, "slug": "blog", "status": "pending", "url": "https://blog.yourdomain.com", "...": "..." },
  "deployment_id": 12,
  "websocket": "/api/ws/apps/1",
  "warnings": ["automatic webhook installation failed: ..."]
}
```

### Applications

| Method | Path | Description |
|---|---|---|
| GET | `/api/apps` | List apps owned by the user |
| GET | `/api/apps/{id}` | App details (+`url`) |
| DELETE | `/api/apps/{id}` | Delete app: container, database + volume, image, GitHub webhook, source, DB row. `204` |
| GET | `/api/apps/{id}/deployments?limit=20` | Deploy history (max 100) |
| POST | `/api/apps/{id}/redeploy` | Redeploy (GitHub source only; ZIP sources must be re-uploaded via `/api/deploy`) |
| GET | `/api/apps/{id}/env` | List env vars |
| PUT | `/api/apps/{id}/env` | Body `{"env": {"KEY":"value"}}`; takes effect on the next deploy |
| DELETE | `/api/apps/{id}/env/{key}` | Delete an env var |

Env var names must match `[A-Za-z_][A-Za-z0-9_]*`. Access to another user's app returns `403`.

### Webhook

`POST /api/webhook/github` (public, secured with HMAC)

- Requires a valid `X-Hub-Signature-256` header; otherwise `401`.
- `ping` → `200 {"status":"pong"}`.
- `push` to `refs/heads/<branch>` → deploys all apps whose `github_full_name` matches, with `auto_deploy=true` and a matching `branch`. Response `202 {"deployments_triggered": N}`. Other branches, branch deletions, and other events are ignored.

## 10. WebSocket (real-time logs)

`GET /api/ws/apps/{id}?token=<session>` — upgrades to a WebSocket. Each app has one room (keyed by slug). A newly connected client immediately receives up to the last 300 events from a running deploy.

All messages are JSON with the common fields `app`, `deployment_id`, `ts` (RFC 3339):

```json
{"type":"log",    "line":"==> Building image paas/blog:12 ..."}
{"type":"status", "status":"building"}      // building | deploying | running | failed
{"type":"done",   "url":"https://blog.yourdomain.com", "image":"paas/blog:12", "duration":"1m12s"}
{"type":"error",  "error":"pack build failed: ..."}
```

Example client:

```js
const ws = new WebSocket(`wss://api.yourdomain.com/api/ws/apps/${id}?token=${token}`);
ws.onmessage = (e) => {
  const ev = JSON.parse(e.data);
  if (ev.type === "log") console.log(ev.line);
};
```

The server sends a ping every 30 seconds. Slow clients may drop messages (256-message buffer per client) so that builds are not blocked. WebSocket origins are restricted by `CORS_ORIGINS`.

## 11. Database schema

Managed by GORM `AutoMigrate` on server start; equivalent SQL migrations are available under `internal/repository/migrations/`.

| Table | Contents |
|---|---|
| `apps` | slug (unique), source (`github`/`zip`), `repo_url`, `branch`, `port`, `container_id`, `image_tag`, `status`, `last_err`, `github_full_name`, `webhook_id`, `auto_deploy`, `owner_login` |
| `deployments` | per deploy: commit sha/message, status, `build_log` (max 1 MiB, tail preserved), `error_msg`, duration, `triggered_by` (`user`/`webhook`) |
| `env_vars` | key/value per app (unique per `app_id,key`) |
| `domains` | hostname & Cloudflare metadata (prepared for custom domains) |
| `user_tokens` | `github_id`, `login`, profile, **encrypted** `access_token`, scopes |
| `app_databases` | isolated database per app: engine, container, volume, host, port, user (password not stored; the URI lives in the `DATABASE_URL` env var) |

## 12. Security

- **GitHub tokens** are encrypted with AES-256-GCM (key derived from `JWT_SECRET`) and are never sent to the client. Changing `JWT_SECRET` makes old tokens unreadable; users must log in again.
- **Clone** uses a temporary `Authorization` header, not a token in the URL, so the token is not persisted in `.git/config` and is scrubbed from error messages. Only `https` URLs without embedded credentials are accepted; branch names are validated.
- **Webhooks** are verified with HMAC-SHA256 using constant-time comparison; without a secret, all requests are rejected.
- **OAuth** uses a random `state` (HttpOnly cookie) to prevent CSRF.
- **ZIP**: zip-slip protection, symlinks ignored, limits of 20,000 files and 1 GiB extracted size, magic-number validation.
- **Authorization**: every app/WS endpoint checks `owner_login`.
- **Query tokens** are only accepted on the WebSocket route.
- App containers are limited to 1 vCPU / 1 GB; database containers to 1 vCPU / 512 MB.
- The backend holds the Docker socket (equivalent to root access on the host). Run it on a dedicated server and restrict who can log in if the platform is not public.
- Env vars are stored as-is in the `env_vars` table (not encrypted); consider encryption if storing sensitive secrets.

## 13. Testing

```bash
go test ./...
```

Tests cover token encryption, sessions, ZIP extraction (including zip-slip attacks), webhook signature verification, OAuth URLs, Traefik labels, repo/branch/env URL validation, log capture, auth middleware, and CORS. Flows that require Docker, `pack`, or a real PostgreSQL instance are not covered by automated tests.

## 14. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Server exits with "required environment variable not set" | Fill in `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, `MAIN_DOMAIN`, `DB_PASSWORD` |
| "JWT_SECRET must be changed in production" | Set `JWT_SECRET` to something other than `change-me` when `APP_ENV=production` |
| `/api/auth/github` → 503 | `GITHUB_CLIENT_ID/SECRET` not set |
| Login succeeds but `/api/github/repos` → 401 | GitHub token missing or `JWT_SECRET` changed; log in again |
| `warnings`: webhook installation failed | The account lacks admin access to the repo, or `GITHUB_WEBHOOK_BASE_URL` is not reachable from the internet; the webhook can be installed manually: Payload URL `…/api/webhook/github`, content type `application/json`, secret = `GITHUB_WEBHOOK_SECRET`, event `push` |
| GitHub webhook returns 401 | The secret on GitHub differs from `GITHUB_WEBHOOK_SECRET` |
| Build fails with "pack: executable not found" | Install `pack` (`scripts/setup.sh`) or use the backend image from `deployments/Dockerfile` |
| Build fails with no Docker access | Ensure `/var/run/docker.sock` is mounted and accessible |
| App runs but the domain doesn't open | Check the tunnel's Public Hostname (`*.domain` → `http://traefik:80`), ensure the container is on `proxy-net`, and confirm the app's `port` matches what the app listens on |
| App crashes due to port | The app must listen on `$PORT` (automatically set to match the app's `port`) |
| Env var changes have no effect | Env vars are only read at deploy time; run a redeploy |
| `503` queue full | Increase `QUEUE_SIZE`/`WORKER_COUNT` or wait for the running deploy to finish |