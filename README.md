# minideploy — Server Deployment Manager

A single-binary tool for deploying applications to a VPS with zero-downtime symlink swaps. Supports **systemd** and **pm2** process managers.

## Quick Start

### 1. Install the daemon on your server

```bash
minideploy init-server --host my-vps --ssh-user root
```

This cross-compiles the daemon, SCPs it to your server, creates a `minideploy` system user, installs a systemd service for the daemon, configures sudoers, and generates an API key.

### 2. Create a `.deploy.yml` in your project

```yaml
app_name: my-api
service_type: systemd
service_name: my-api@%i
instances:
  - id: "3000"
    port: 3000
deploy_path: /opt/my-api
build:
  - go build -o app .
artifacts:
  - app
server:
  host: my-vps
  api_port: 8443
  ssh_user: root
  api_key: <from init-server output>
```

See `.deploy.yml.example` for a full reference.

### 3. Deploy

```bash
minideploy deploy
```

This runs your build steps, rsyncs artifacts to `/opt/my-api/upload/`, and tells the daemon to snapshot, symlink, and restart your service.

## CLI Reference

| Command | Description |
|---|---|
| `daemon` | Start the daemon (run as systemd service) |
| `deploy` | Full pipeline: build → upload → deploy |
| `build` | Run build steps only |
| `upload` | Rsync artifacts only |
| `rollback [release]` | Rollback to a previous or specified release |
| `status` | Daemon health check |
| `ps` | List running apps and instances |
| `releases [app]` | List releases for an app |
| `logs [app]` | Tail app logs |
| `init-server` | Bootstrap daemon on a fresh VPS |

## How It Works

```
[Dev Machine]                          [VPS]
  .deploy.yml                              ┌──────────────┐
  ┌──────────┐  1. Build                  │  minideploy  │
  │  Client  │  ──► compile, test, etc.   │  Daemon      │
  │   CLI    │  2. rsync ────────────────► │  (REST API)  │
  └──────────┘  3. POST /deploy ─────────► │              │
                  Bearer <key>             │  copy →      │
                                           │  releases/   │
                                           │  symlink     │
                                           │  restart     │
                                           └──────────────┘
```

### Server directory layout

```
/opt/<app_name>/
├── upload/                        # rsync target (incremental)
├── releases/
│   ├── 20260603-143022/           # immutable snapshot
│   └── 20260602-120000/
├── current → releases/20260603-143022/  # active release
```

## API Key Resolution

Priority (highest to lowest):
1. `server.api_key` in `.deploy.yml`
2. `MINIDEPLOY_API_KEY` environment variable
3. `MINIDEPLOY_API_KEY` in `.env` file in project root

This lets you commit `.deploy.yml` without exposing secrets.
