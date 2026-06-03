# minideploy — Server Deployment Manager

## Overview

A Go-based deployment tool with a client-side CLI and server-side daemon. The client builds a project, uploads artifacts via rsync, then triggers the daemon to snapshot the release, symlink it, and restart the service (systemd or pm2).

## Architecture

```
[Dev Machine]                          [VPS]
  .deploy.yml                              ┌──────────────────┐
  ┌──────────┐  1. Build                  │  minideploy      │
  │  Client  │  ──► compile, test, etc.   │  Daemon          │
  │   CLI    │  2. rsync ───────────────► │  (REST API)      │
  └──────────┘  3. POST /deploy ────────► │  3. copy →       │
                  Authorization: Bearer    │     releases/    │
                                          │  4. symlink      │
                                          │  5. restart      │
                                          │     systemd/pm2  │
                                          └──────────────────┘
```

## Binary

Single binary `minideploy` with subcommands (like `docker`, `kubectl`).

## CLI Commands

| Command | Role | Description |
|---|---|---|
| `minideploy daemon` | **Server** | Run the daemon |
| `minideploy deploy` | **Client** | Full pipeline: build → rsync → API deploy |
| `minideploy build` | **Client** | Execute build steps only |
| `minideploy upload` | **Client** | Rsync artifacts only |
| `minideploy rollback` | **Client** | Rollback to a previous release |
| `minideploy status` | **Client** | Daemon health & overview |
| `minideploy ps` | **Client** | List all running apps/instances |
| `minideploy releases` | **Client** | List releases for an app |
| `minideploy logs` | **Client** | Tail app logs |
| `minideploy init-server` | **Both** | Bootstrap daemon on a fresh VPS |

## Project Structure

```
minideploy/
├── main.go                           # cobra root command
├── cmd/
│   ├── deploy.go
│   ├── build.go
│   ├── upload.go
│   ├── rollback.go
│   ├── status.go
│   ├── ps.go
│   ├── releases.go
│   ├── logs.go
│   ├── daemon.go
│   └── init_server.go
├── internal/
│   ├── client/
│   │   ├── config.go                # YAML loading & validation
│   │   ├── build.go                 # Sequential build step runner
│   │   ├── rsync.go                 # rsync command construction & exec
│   │   ├── tunnel.go                # SSH tunnel lifecycle
│   │   └── api_client.go            # HTTP client for daemon API
│   ├── daemon/
│   │   ├── server.go                # HTTP server setup & graceful shutdown
│   │   ├── router.go                # Route registration
│   │   ├── middleware.go            # Auth (Bearer token), logging, recovery
│   │   ├── handlers.go              # All API handlers
│   │   ├── deploy.go                # Core deploy logic (copy, symlink)
│   │   ├── process.go               # ProcessManager interface + impls
│   │   ├── auth.go                  # API key generation, bcrypt, validation
│   │   └── state.go                 # JSON file state persistence
│   └── shared/
│       ├── types.go                 # Shared types
│       └── ssh_config.go            # ~/.ssh/config parser
├── go.mod
├── Makefile
└── .deploy.yml.example
```

## YAML Config (`.deploy.yml`)

```yaml
app_name: my-api
service_type: systemd              # "systemd" or "pm2"
service_name: my-api@%i            # %i replaced per instance
instances:
  - id: "3000"
    port: 3000
    env:
      PORT: 3000
  - id: "3001"
    port: 3001
    env:
      PORT: 3001
deploy_path: /opt/my-api
build:
  - go build -o app .
  - npm run build --prefix frontend
artifacts:
  - app
  - frontend/build/
server:
  host: my-vps                     # hostname (reads ~/.ssh/config) or IP
  api_port: 8443
  ssh_user: root
  api_key: sk-abc123...            # optional — read from env if missing
env:
  NODE_ENV: production
pre_deploy: []                     # future: run on server before symlink
post_deploy: []                    # future: run on server after restart
```

The `api_key` field reads from environment variables/env file if not specified:
1. `server.api_key` in `.deploy.yml`
2. `MINIDEPLOY_API_KEY` env var
3. `.env` file in project root (`MINIDEPLOY_API_KEY=sk-...`)

## Server-Side Disk Layout

```
/opt/<app_name>/
├── upload/                        # rsync target (incremental)
├── releases/                      # Immutable release snapshots
│   ├── 20260603-143022/
│   │   ├── app
│   │   └── frontend/build/
│   └── 20260602-120000/
├── current → releases/20260603-143022/
```

Daemon state at `/var/lib/minideploy/`:
```
/var/lib/minideploy/
├── state.json                     # Persisted app configs, releases, keys
└── daemon.log
```

## REST API

Base: `http://127.0.0.1:8443/api/v1/`
Auth: `Authorization: Bearer <api_key>`

| Method | Path | Description |
|---|---|---|
| `POST` | `/deploy` | Deploy (copy upload→release, symlink, restart) |
| `POST` | `/rollback` | Rollback to previous or specified release |
| `GET` | `/status` | Daemon health & disk usage |
| `GET` | `/apps` | List all registered apps |
| `GET` | `/apps/:name` | App detail (instances, current release, config) |
| `GET` | `/apps/:name/status` | Per-app process status |
| `GET` | `/apps/:name/releases` | Release history |
| `GET` | `/apps/:name/logs` | Tail app logs |
| `GET` | `/health` | Simple health check |

## Auth Flow

1. `init-server` generates a 32-byte random hex key
2. Stores bcrypt hash in `state.json`
3. Prints raw key once (user saves it to `.deploy.yml` or `.env`)
4. Client sends `Authorization: Bearer <key>` on all requests
5. Daemon middleware: `bcrypt.CompareHashAndPassword(stored_hash, provided_key)`

## Process Manager Abstraction

```go
type ProcessManager interface {
    Restart(ctx, serviceName, instanceID) error
    Start(ctx, serviceName, instanceID) error
    Stop(ctx, serviceName, instanceID) error
    Status(ctx, serviceName, instanceID) (ProcessStatus, error)
    Logs(ctx, serviceName, instanceID, lines int) (string, error)
}
```

- **SystemdManager**: uses `sudo systemctl <action> <unit>`
- **PM2Manager**: uses `sudo pm2 <action> <name>`
- Selected at deploy time based on `service_type`

## Deploy Flow (end-to-end)

```
1. Parse .deploy.yml
2. Execute build steps (fail on first error)
3. Verify artifacts exist on disk
4. Resolve server.host (DNS → SSH config alias lookup)
5. rsync -avz --delete artifacts/ ssh_user@host:/opt/app/upload/
6. Determine if SSH tunnel needed (SSH alias → start tunnel)
7. POST /api/v1/deploy with release_name (auto or --release flag)
8. Daemon: copy upload/ → releases/<name>/, atomic symlink swap, restart instances
9. Print result
```

## Rollback

```
POST /api/v1/rollback { app_name, release_name? }
→ Daemon looks up previous release from state, swaps symlink, restarts
```

## Privilege Model

- **`init-server`** (run as root): create `minideploy` user, install binary, create dirs, write sudoers
- **`minideploy` user** (daemon runtime): write to `/opt/<app>` and `/var/lib/minideploy/`, run `systemctl`/`pm2` via sudo
- **Sudoers**: narrow rules for `systemctl`, `journalctl`, `useradd` only

## Implementation Order

### Phase 1 — Skeleton & Shared
- go.mod init
- main.go with cobra root
- shared/types.go (all shared structs)
- client/config.go (YAML loader with env fallback for api_key)
- Makefile

### Phase 2 — Daemon Core
- daemon/state.go (JSON state persistence)
- daemon/auth.go (key generation, bcrypt)
- daemon/middleware.go (Bearer auth, logging, recovery)
- daemon/process.go (ProcessManager interface + systemd/pm2)
- daemon/deploy.go (copy, symlink logic)
- daemon/handlers.go (all REST handlers)
- daemon/router.go (route registration)
- daemon/server.go (HTTP server + graceful shutdown)
- cmd/daemon.go

### Phase 3 — Client Core
- client/ssh_config.go (~/.ssh/config parser)
- client/build.go (build step executor)
- client/rsync.go (rsync command)
- client/tunnel.go (SSH tunnel)
- client/api_client.go (HTTP client)
- shared/ssh_config.go

### Phase 4 — CLI Commands
- cmd/deploy.go (orchestrator)
- cmd/build.go, upload.go, rollback.go
- cmd/status.go, ps.go, releases.go, logs.go
- cmd/init_server.go

### Phase 5 — Polish
- Error handling, colored CLI output
- .deploy.yml.example
- README.md
- Makefile install targets

## Error Handling Principles

- State file corruption: detect on read, backup corrupt file, refuse to continue
- Rsync failure: don't call deploy API, print full stderr
- systemd restart failure: log, continue remaining instances, report partial failure
- API key mismatch: 401 with generic "unauthorized" message
- Symlink: use os.Rename for atomicity where possible
