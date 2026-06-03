# CLI Reference

minideploy is a single binary with subcommands. Use `minideploy --help` to list all commands.

## Global Flags

| Flag | Description |
|---|---|
| `--help` | Show help for any command |

## `minideploy deploy`

Run the full deployment pipeline: build, rsync, and trigger the daemon deploy.

```
minideploy deploy [--config path] [--release name]
```

| Flag | Default | Description |
|---|---|---|
| `-c, --config` | `.deploy.yml` | Path to config file |
| `-r, --release` | auto-generated | Custom release name (`YYYYMMDD-HHMMSS` format) |

**Flow**: Run build steps → verify artifacts → rsync to server → POST /deploy → print result.

**Example**:

```bash
$ minideploy deploy
[deploy] starting deployment for my-api
[build] (1/2) go build -o app .
[build] (2/2) npm run build --prefix frontend
[build] all 2 steps completed
[rsync] rsync -avz --delete app frontend/build/ root@my-vps:/opt/my-api/upload/
sending incremental file list
app
frontend/build/index.html
              sent 8.2M bytes  received 48 bytes  1.6M bytes/sec

[deploy] release 20260603-143022 deployed successfully
[deploy] instances restarted: [my-api@3000 my-api@3001]
```

## `minideploy build`

Run build steps only. Useful for testing your build configuration.

```
minideploy build [--config path]
```

| Flag | Default | Description |
|---|---|---|
| `-c, --config` | `.deploy.yml` | Path to config file |

## `minideploy upload`

Rsync artifacts to the server upload directory without triggering a deploy.

```
minideploy upload [--config path]
```

| Flag | Default | Description |
|---|---|---|
| `-c, --config` | `.deploy.yml` | Path to config file |

## `minideploy rollback [release]`

Rollback the symlink to a previous release and restart all service instances.

```
minideploy rollback [release-name] [--config path]
```

| Argument | Description |
|---|---|
| `release-name` | Optional. Rollback to this specific release. Omit to rollback to the previous release. |

| Flag | Default | Description |
|---|---|---|
| `-c, --config` | `.deploy.yml` | Path to config file |

**How it finds the previous release**: The daemon lists all directories in the `releases/` folder, reads the `current` symlink target, and picks the most recent release that isn't the current one.

**Example**:

```bash
$ minideploy rollback
[rollback] rolled back to release 20260602-120000
[rollback] instances restarted: [my-api@3000 my-api@3001]

$ minideploy rollback 20260601-100000
[rollback] rolled back to release 20260601-100000
[rollback] instances restarted: [my-api@3000 my-api@3001]
```

## `minideploy destroy [app-name]`

Remove an app from the daemon. Requires `--confirm`.

```
minideploy destroy [app-name] [--config path] [--soft] --confirm
```

| Flag | Default | Description |
|---|---|---|
| `-y, --confirm` | `false` | Acknowledge the destruction (required) |
| `-s, --soft` | `false` | Keep files on disk, just unregister |
| `-c, --config` | `.deploy.yml` | Path to config file |

**Hard destroy** (default): Stops all services, removes the entire `deploy_path` directory, and unregisters the app.

**Soft destroy** (`--soft`): Stops all services and unregisters the app, but leaves `/opt/<app>/` on disk in case you need to recover a release.

**Examples**:

```bash
# Hard destroy (removes everything)
$ minideploy destroy --confirm

# Soft destroy (keeps files)
$ minideploy destroy --soft --confirm

# Specify app name explicitly
$ minideploy destroy my-api --confirm
```

## `minideploy status`

Query the daemon for server health information.

```
minideploy status [--host HOST] [--port PORT] [--api-key KEY]
```

| Flag | Default | Description |
|---|---|---|
| `-H, --host` | `127.0.0.1` | Daemon host |
| `-p, --port` | `8443` | Daemon API port |
| `-k, --api-key` | env or empty | API key |

**Example**:

```bash
$ minideploy status
Daemon:  minideploy v0.1.0
Uptime:  3h12m45s
Apps:    2
```

## `minideploy ps`

List all registered apps and their running instances.

```
minideploy ps [--host HOST] [--port PORT] [--api-key KEY]
```

| Flag | Default | Description |
|---|---|---|
| `-H, --host` | `127.0.0.1` | Daemon host |
| `-p, --port` | `8443` | Daemon API port |
| `-k, --api-key` | env or empty | API key |

**Example**:

```bash
$ minideploy ps
my-api              running   20260603-143022
  └─ my-api@3000  ●  (port 3000)
  └─ my-api@3001  ●  (port 3001)
auth-service        running   20260602-100000
  └─ auth@8080    ●  (port 8080)
```

## `minideploy releases [app-name]`

List all releases for an app.

```
minideploy releases [app-name] [--host HOST] [--port PORT] [--api-key KEY]
```

| Flag | Default | Description |
|---|---|---|
| `-H, --host` | `127.0.0.1` | Daemon host |
| `-p, --port` | `8443` | Daemon API port |
| `-k, --api-key` | env or empty | API key |

**Example**:

```bash
$ minideploy releases my-api
→ 20260603-143022  2026-06-03 14:30:22
  20260602-120000  2026-06-02 12:00:00
  20260601-090000  2026-06-01 09:00:00
```

The `→` marks the current active release.

## `minideploy logs [app-name]`

Fetch the most recent log entries for all instances of an app.

```
minideploy logs [app-name] [--host HOST] [--port PORT] [--api-key KEY]
```

| Flag | Default | Description |
|---|---|---|
| `-H, --host` | `127.0.0.1` | Daemon host |
| `-p, --port` | `8443` | Daemon API port |
| `-k, --api-key` | env or empty | API key |

**Example**:

```bash
$ minideploy logs my-api
--- my-api@3000 ---
Jun 03 14:30:22 vps systemd[1]: Started my-api@3000.service
Jun 03 14:30:22 vps my-api[1234]: Server listening on port 3000

--- my-api@3001 ---
Jun 03 14:30:22 vps systemd[1]: Started my-api@3001.service
Jun 03 14:30:22 vps my-api[1235]: Server listening on port 3001
```

## `minideploy daemon`

Start the minideploy daemon. Intended to be run as a systemd service.

```
minideploy daemon [--port PORT] [--state-dir DIR]
```

| Flag | Default | Description |
|---|---|---|
| `-p, --port` | `8443` | Port to listen on |
| `-d, --state-dir` | `/var/lib/minideploy` | State directory |

The daemon:
- Listens on `127.0.0.1:<port>` (localhost only)
- Generates an API key on first start if none exists
- Persists app state to `state.json` in the state directory
- Requires `sudo` access for process manager commands

## `minideploy init`

Interactively generate a `.deploy.yml` file in the current directory.

```
minideploy init [--force]
```

| Flag | Default | Description |
|---|---|---|
| `-f, --force` | `false` | Overwrite existing `.deploy.yml` |

Prompts for all fields with sensible defaults:

```
$ minideploy init
minideploy init — generating .deploy.yml
----------------------------------------
App name [my-app]:
Service type (systemd/pm2) [systemd]:
Service name (use %i for instances) [my-api@%i]:
Number of instances [1]: 2
  Instance 1 port [3000]:
  Instance 2 port [3001]:
Deploy path [/opt/my-api]:
Build steps (one per line, empty line to finish):
  Step 1: go build -o app .
  Step 2:
Artifacts to upload (one per line, empty line to finish):
  Artifact 1: app
  Artifact 2:
Server host: my-vps
Server API port [8443]:
SSH user [root]:
API key (leave blank for env/MINIDEPLOY_API_KEY):
Environment variables (KEY=VALUE, one per line, empty to finish):
  Env 1: NODE_ENV=production
  Env 2:
----------------------------------------
.deploy.yml generated successfully!
```

## `minideploy init-server`

Bootstrap the minideploy daemon on a fresh VPS.

```
minideploy init-server --host HOST [--ssh-user USER] [--app-name NAME] [--deploy-path PATH]
```

| Flag | Default | Description |
|---|---|---|
| `--host` | (required) | VPS hostname or IP |
| `--ssh-user` | `root` | SSH user for initial setup |
| `--app-name` | `my-app` | Default app name to create directories for |
| `--deploy-path` | `/opt/<app-name>` | Deploy path on server |

The command:
1. Cross-compiles the daemon for `linux/amd64`
2. SCPs the binary to `/usr/local/bin/minideploy`
3. Creates the `minideploy` system user
4. Sets up directory structure
5. Installs a systemd service for the daemon
6. Configures sudoers for the `minideploy` user
7. Generates and displays an API key
8. Starts the daemon
