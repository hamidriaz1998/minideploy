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

# Number of old releases to keep on disk (0 = keep all).
# Oldest releases are pruned after a successful deploy.
keep_releases: 5

# Health check configuration for zero-downtime verification.
# If the endpoint doesn't respond 2xx/3xx within retries,
# the deploy is automatically rolled back to the previous release.
health_check:
  endpoint: /health       # HTTP path to check on each instance
  timeout: 10             # Seconds per request
  retries: 3              # Retries per instance before marking failed
  wait_between_instances: 1  # Seconds between checking instances

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
| 4 | Host-specific key in global config | `~/.config/minideploy/config.yml` → `Hosts[server.host]` |

For admin operations (key management, destroy), the key is resolved as:

| Priority | Source | Example |
|---|---|---|
| 1 | `--api-key` flag | `--api-key sk-abc123...` |
| 2 | `MINIDEPLOY_API_KEY` env var | `export MINIDEPLOY_API_KEY=sk-abc...` |
| 3 | Host-specific key in global config | `~/.config/minideploy/config.yml` → `Hosts[host]` |
| 4 | Legacy `admin_key` in global config | (written by older versions) |

This means you can commit `.deploy.yml` without secrets by:

```bash
# .env file (gitignored)
MINIDEPLOY_API_KEY=sk-abc123def456...

# Or export it
export MINIDEPLOY_API_KEY=sk-abc123def456...
```

## Global Client Config

minideploy stores per-host API keys in a global config file at `~/.config/minideploy/config.yml`:

```yaml
my-vps:
  admin_key: sk-abc123def456...
test-vm:
  admin_key: sk-789def...
```

Each host gets its own top-level key, set automatically by `init-server`:

```bash
minideploy init-server --host my-vps    # writes my-vps.admin_key
minideploy init-server --host test-vm   # writes test-vm.admin_key (no overwrite)
```

The key for a host is used as a fallback for `deploy`, `status`, `logs`, and admin operations like `create-key`, `delete-key`, and `keys`.

For key management commands (`keys`, `create-key`, `delete-key`), the host is resolved from `.deploy.yml` first, then from the `--host` flag. If neither is set, the command errors out with instructions.

You can view the raw key:

```bash
minideploy config get admin_key
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
| `keep_releases` | Non-negative integer; `0` = keep all |
| `health_check.endpoint` | Optional; if set, `timeout`, `retries`, and `wait_between_instances` must be valid |
| `health_check.timeout` | Must be positive if health_check is configured |
| `health_check.retries` | Must be positive if health_check is configured |
| `health_check.wait_between_instances` | Cannot be negative |

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
