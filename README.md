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

See `.deploy.yml.example` for a full reference. Or generate one interactively:

```bash
minideploy init
```

For IDE autocompletion and validation, add the [JSON Schema](deploy-schema.json) to your editor:

<details>
<summary><b>VS Code / Cursor / Windsurf</b></summary>

In `.vscode/settings.json`:

```json
{
  "yaml.schemas": {
    "https://raw.githubusercontent.com/<your-repo>/main/deploy-schema.json": [
      ".deploy.yml"
    ]
  }
}
```

Or add this comment at the top of your `.deploy.yml`:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/<your-repo>/main/deploy-schema.json
```
</details>

<details>
<summary><b>Neovim (with lspconfig + yamlls)</b></summary>

```lua
require('lspconfig').yamlls.setup({
  settings = {
    yaml = {
      schemas = {
        ["https://raw.githubusercontent.com/<your-repo>/main/deploy-schema.json"] = ".deploy.yml",
      },
    },
  },
})
```
</details>

<details>
<summary><b>IntelliJ IDEA / JetBrains</b></summary>

1. Open **Settings → Languages & Frameworks → Schemas and DTDs → JSON Schema Mappings**
2. Add a mapping:
   - **Schema file or URL**: `https://raw.githubusercontent.com/<your-repo>/main/deploy-schema.json`
   - **File path pattern**: `.deploy.yml`
</details>

Replace `<your-repo>` with the actual repository URL after pushing.

### 3. Deploy

```bash
minideploy deploy
```

This runs your build steps, rsyncs artifacts to `/opt/my-api/upload/`, and tells the daemon to snapshot, symlink, and restart your service.

## CLI Reference

| Command | Description |
|---|---|
| `deploy` | Full pipeline: build → upload → deploy |
| `build` | Run build steps only |
| `upload` | Rsync artifacts only |
| `rollback [release]` | Rollback to a previous or specified release |
| `destroy [app]` | Remove an app (soft or hard) |
| `status` | Daemon health check |
| `ps` | List running apps and instances |
| `releases [app]` | List releases for an app |
| `logs [app]` | Tail app logs |
| `rotate-key` | Generate a new API key |
| `init` | Generate a `.deploy.yml` interactively |
| `init-server` | Bootstrap daemon on a fresh VPS |
| `daemon` | Start the daemon (run as systemd service) |

## Documentation

Detailed docs are in the [`docs/`](docs/index.md) directory:

| Topic | File |
|---|---|
| Architecture | [01-overview.md](docs/01-overview.md) |
| Installation | [02-installation.md](docs/02-installation.md) |
| Configuration | [03-configuration.md](docs/03-configuration.md) |
| CLI Reference | [04-cli-reference.md](docs/04-cli-reference.md) |
| Deployment Walkthrough | [05-deployment.md](docs/05-deployment.md) |
| Process Managers | [06-process-managers.md](docs/06-process-managers.md) |
| Rollback & Destroy | [07-rollback-destroy.md](docs/07-rollback-destroy.md) |
| API Reference | [08-api-reference.md](docs/08-api-reference.md) |
| Security | [09-security.md](docs/09-security.md) |
| Server Layout | [10-server-layout.md](docs/10-server-layout.md) |
| Troubleshooting | [11-troubleshooting.md](docs/11-troubleshooting.md) |

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
