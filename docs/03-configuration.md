# Configuration

## The `.deploy.yml` File

minideploy reads its configuration from a `.deploy.yml` file in your project root. Use `minideploy init` to generate one interactively.

## Full Schema

```yaml
# ── Required ──────────────────────────────────────────────

# Name of your application. Used for state tracking and log messages.
app_name: my-api

# Process manager type: "systemd" or "pm2"
service_type: systemd

# Service name pattern. Use %i as a placeholder for the instance ID.
# This maps to systemd unit names or pm2 app names.
# For systemd: "my-api@%i" → restarts "my-api@3000.service"
# For pm2:     "my-api@%i" → restarts pm2 app named "my-api@3000"
service_name: my-api@%i

# List of service instances to manage.
# Each instance gets its own systemd unit or pm2 process.
instances:
  - id: "3000"              # Replaces %i in service_name
    port: 3000
    env:                     # Instance-specific env vars
      PORT: 3000
  - id: "3001"
    port: 3001
    env:
      PORT: 3001

# Base path on the server where releases are stored.
# The daemon creates upload/, releases/, and current/ under this path.
deploy_path: /opt/my-api

# Build steps executed sequentially on the development machine.
# Each step runs via "sh -c <step>". Fails on first non-zero exit.
build:
  - go build -o app .
  - npm run build --prefix frontend

# Files and directories to upload to the server.
# Paths are relative to the project root.
# These are passed directly to rsync.
artifacts:
  - app
  - frontend/build/

# ── Server ─────────────────────────────────────────────────

server:
  # Server hostname or IP address.
  # If it's a hostname that doesn't resolve via DNS, minideploy
  # reads ~/.ssh/config to find the HostName.
  host: my-vps

  # Port the daemon API listens on (default: 8443)
  api_port: 8443

  # SSH user for rsync connections
  ssh_user: root

  # API key for authenticating with the daemon.
  # Optional — see "API Key Resolution" below.
  api_key: sk-abc123...

# ── Optional ───────────────────────────────────────────────

# Global environment variables passed to all instances.
# These are merged with (and can be overridden by) instance-specific env.
env:
  NODE_ENV: production
  LOG_LEVEL: info

# Pre-deploy hooks run on the server AFTER the symlink swap
# but BEFORE the service restart. (Future feature — reserved)
# pre_deploy:
#   - cmd: make migrate

# Post-deploy hooks run on the server AFTER the service restart.
# (Future feature — reserved)
# post_deploy:
#   - cmd: make warmup
```

## API Key Resolution

The API key for deploy/status/logs operations is resolved in this priority order (highest to lowest):

| Priority | Source | Example |
|---|---|---|
| 1 | `server.api_key` in `.deploy.yml` | `api_key: sk-abc123...` |
| 2 | `MINIDEPLOY_API_KEY` env var | `export MINIDEPLOY_API_KEY=sk-abc...` |
| 3 | `.env` file in project root | `MINIDEPLOY_API_KEY=sk-abc...` |

For admin operations (key management, destroy), the key is resolved as:

| Priority | Source | Example |
|---|---|---|
| 1 | `--api-key` flag | `--api-key sk-abc123...` |
| 2 | `MINIDEPLOY_API_KEY` env var | `export MINIDEPLOY_API_KEY=sk-abc...` |
| 3 | `~/.config/minideploy/config.yml` → `admin_key` | (set via `init-server` or `config set admin_key`) |

This means you can commit `.deploy.yml` without secrets by:

```bash
# .env file (gitignored)
MINIDEPLOY_API_KEY=sk-abc123def456...

# Or export it
export MINIDEPLOY_API_KEY=sk-abc123def456...
```

## Global Client Config

minideploy stores your admin API key in a global config file at `~/.config/minideploy/config.yml`:

```yaml
admin_key: sk-abc123def456...
```

This is automatically populated by `minideploy init-server` and used as a fallback for admin operations like `create-key`, `delete-key`, `keys`, `rotate-key`, and `destroy`.

You can manage it manually:

```bash
# View the stored admin key
minideploy config get admin_key

# Update it
minideploy config set admin_key sk-newkey...
```

## SSH Configuration

When `server.host` is a hostname (not an IP address), minideploy attempts to resolve it:

1. **DNS lookup** — if the hostname resolves, it's used directly
2. **SSH config fallback** — if DNS fails, `~/.ssh/config` is parsed to find the `HostName`, `User`, `Port`, and `IdentityFile`

Example `~/.ssh/config`:

```
Host my-vps
    HostName 203.0.113.42
    User root
    Port 2222
    IdentityFile ~/.ssh/vps-key
```

This allows rsync to work seamlessly with SSH host aliases. The daemon API, however, requires network connectivity to the resolved host.

## Generating a Config

```bash
# Interactive wizard
minideploy init

# Overwrite existing
minideploy init --force
```

The wizard prompts for all fields with sensible defaults, collecting build steps and artifacts one per line. See the [Deployment Walkthrough](05-deployment.md) for examples.

## Validation Rules

The config loader enforces these rules:

| Field | Rule |
|---|---|
| `app_name` | Required, non-empty |
| `service_type` | Must be `systemd` or `pm2` |
| `service_name` | Required, non-empty |
| `deploy_path` | Required, non-empty |
| `server.host` | Required, non-empty |
| `server.api_port` | Defaults to `8443` |
| `build` | At least one step required |
| `artifacts` | At least one artifact required |

## IDE Integration (JSON Schema)

A [JSON Schema file](../deploy-schema.json) is provided for autocompletion and validation in YAML-aware editors.

**VS Code** — add to `.vscode/settings.json`:

```json
{
  "yaml.schemas": {
    "https://raw.githubusercontent.com/hamidriaz1998/minideploy/main/deploy-schema.json": [".deploy.yml"]
  }
}
```

**Or** add this comment to the top of `.deploy.yml`:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/hamidriaz1998/minideploy/main/deploy-schema.json
```

The URL points to the `main` branch — adjust if you're using a different branch.
