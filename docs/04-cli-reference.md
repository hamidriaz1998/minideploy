# CLI Reference

minideploy is a single binary with subcommands. Use `minideploy --help` to list all commands.

| Command | Description |
|---|---|
| `deploy` | Build, upload, and deploy |
| `build` | Run build steps only |
| `upload` | Rsync artifacts only |
| `rollback` | Rollback to a previous release |
| `destroy` | Remove an app from the daemon |
| `status` | Daemon health information |
| `ps` | List apps and running instances |
| `releases` | List releases for an app |
| `logs` | Fetch app logs |
| `init` | Generate `.deploy.yml` |
| `init-server` | Bootstrap daemon on a VPS |
| `daemon` | Start the daemon |
| `rotate-key` | Generate a new API key |
| `config` | Manage global config (`get`/`set`) |
| `create-key` | Create a scoped API key |
| `delete-key` | Delete an API key |
| `keys` | List all API keys |
| `completion` | Generate shell completion scripts |

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

## `minideploy rotate-key`

Generate a new API key for the daemon.

```
minideploy rotate-key [--revoke-old] [--config path] [--host HOST] [--port PORT] [--api-key KEY]
```

| Flag | Default | Description |
|---|---|---|
| `--revoke-old` | `false` | Invalidate all previous keys immediately |
| `-c, --config` | `.deploy.yml` | Path to config file |
| `-H, --host` | `127.0.0.1` | Daemon host |
| `-p, --port` | `8443` | Daemon API port |
| `-k, --api-key` | env or config | Current API key for authentication |

By default, old keys remain valid after rotation so you can update CI/CD pipelines and team members at your own pace.

> **Note**: Only global-scoped (admin) keys can rotate keys. App-scoped keys are not permitted.

**Examples**:

```bash
# Rotate key (old keys still work)
$ minideploy rotate-key
New API key: a1b2c3d4e5f6...
Active keys: 2
Old keys remain valid. Use --revoke-old to invalidate them.

# Rotate and revoke all previous keys
$ minideploy rotate-key --revoke-old
New API key: f6e5d4c3b2a1...
Active keys: 1
Old keys have been revoked.
```

## `minideploy config`

Manage the global minideploy configuration at `~/.config/minideploy/config.yml`.

### `minideploy config get <key>`

Get a stored config value.

```
minideploy config get admin_key
```

### `minideploy config set <key> <value>`

Set a config value.

```
minideploy config set admin_key sk-abc123...
```

**Example**:

```bash
$ minideploy config get admin_key
sk-abc123def456...

$ minideploy config set admin_key sk-newkey...
config admin_key updated
```

## `minideploy create-key`

Create a new API key with a specific scope (global or app) and optional label.

```
minideploy create-key [--scope SCOPE] [--app-name NAME] [--label LABEL]
                      [--host HOST] [--port PORT] [--api-key KEY]
```

| Flag | Default | Description |
|---|---|---|
| `--scope` | `app` | Key scope: `global` or `app` |
| `-a, --app-name` | `""` | App name (required for `--scope app`) |
| `-l, --label` | `""` | Optional human-readable label |
| `-H, --host` | `127.0.0.1` | Daemon host |
| `-p, --port` | `8443` | Daemon API port |
| `-k, --api-key` | config or env | Admin API key for authentication |

**Global keys** have full access to all apps and all operations (deploy, destroy, key management).

**App-scoped keys** can only deploy, view status, and fetch logs for a single app. They cannot call `rotate-key`, `destroy`, `create-key`, `delete-key`, or `keys`.

**Examples**:

```bash
# Create an app-scoped key for "my-api"
$ minideploy create-key --app-name my-api --label "CI/CD deploy key"
Key created (id=2)
Scope:   app
App:     my-api
Label:   CI/CD deploy key
API Key: b2c3d4e5f6a7...

# Create a global admin key
$ minideploy create-key --scope global --label "admin backup key"
Key created (id=3)
Scope:   global
Label:   admin backup key
API Key: c3d4e5f6a7b8...
```

## `minideploy delete-key <id>`

Permanently delete an API key by its ID.

```
minideploy delete-key <id> [--host HOST] [--port PORT] [--api-key KEY]
```

Use `minideploy keys` to find key IDs.

**Example**:

```bash
$ minideploy delete-key 2
Key 2 deleted
```

## `minideploy keys`

List all API keys registered with the daemon.

```
minideploy keys [--host HOST] [--port PORT] [--api-key KEY]
```

Shows the key ID, scope, associated app, label, and creation date.

**Example**:

```bash
$ minideploy keys
ID   Scope    App               Label                Created
---- -------- ---------------- -------------------- ------------
1    global   (all)            initial key          2026-06-03
2    app      my-api            CI/CD deploy key    2026-06-04
3    global   (all)             admin backup key    2026-06-04
```

## `minideploy init`

Interactively generate a `.deploy.yml` file in the current directory.

```
minideploy init [--force]
```

| Flag | Default | Description |
|---|---|---|
| `-f, --force` | `false` | Overwrite existing `.deploy.yml` |

Prompts for all fields with sensible defaults across multiple screens (use arrow keys, Tab, and Enter):

```
$ minideploy init
┌─────────────────────────────────────────────────────────────────┐
│ App name                                                       │
│ Name of your application                                       │
│                                                                │
│ my-api________________________________________________________ │
│                                                                │
│ Service type                                                   │
│ Process manager to use                                         │
│ ◉ systemd                                                      │
│ ○ pm2                                                          │
│                                                                │
│ Service name                                                   │
│ Use %i as a placeholder for the instance ID                    │
│                                                                │
│ my-api@%i_____________________________________________________ │
│                                                                │
│ Number of instances                                            │
│ How many service instances to run                              │
│                                                                │
│ 2_____________________________________________________________ │
│                                                                │
│ Start port                                                     │
│ First instance port; subsequent instances increment by 1       │
│                                                                │
│ 3000__________________________________________________________ │
│                                                                │
├─────────────────────────────────────────────────────────────────┤
│ next (tab)                                      (ctrl+c) quit  │
└─────────────────────────────────────────────────────────────────┘
```

Use `Tab` to move between fields, `Enter` to go to the next page, and fill multi-line text fields (build steps, artifacts) with one item per line.

**Example multi-line input**:
```
go build -o app .
npm run build --prefix frontend
```

## `minideploy init-server`

Bootstrap the minideploy daemon on a fresh VPS.

```
minideploy init-server --host HOST [--ssh-user USER] [--app-name NAME] [--deploy-path PATH]
```

The admin API key is automatically saved to `~/.config/minideploy/config.yml` so
you can immediately run admin commands like `create-key` without passing `--api-key`.

**Output**:

```
═══════════════════════════════════════════
  Daemon installed!

  Host:      my-vps
  API Port:  8443

  Admin API Key (saved to global config):
  a1b2c3d4e5f6... (64 hex chars)

  Add to .deploy.yml:
  server:
    host: my-vps
    api_port: 8443
    ssh_user: root
    api_key: a1b2c3d4e5f6...

  Or create app-scoped keys with:
  minideploy create-key --scope app --app-name <name>
═══════════════════════════════════════════
```



| Flag | Default | Description |
|---|---|---|---|
| `--host` | (required) | VPS hostname or IP |
| `--ssh-user` | `root` | SSH user for initial setup |
| `--app-name` | `my-app` | Default app name to create directories for |
| `--deploy-path` | `/opt/<app-name>` | Deploy path on server |
| `-b, --binary` | running binary | Path to minideploy binary to upload |

By default, the running binary (`os.Executable()`) is uploaded to the server. Use `--binary /path/to/minideploy` to upload a different version.

The command:
1. Uploads the minideploy binary to `/usr/local/bin/minideploy`
2. Creates the `minideploy` system user
3. Sets up directory structure
4. Installs a systemd service for the daemon
5. Configures sudoers for the `minideploy` user
6. Generates and displays an API key
7. Saves the admin key to `~/.config/minideploy/config.yml`
8. Starts the daemon

## `minideploy completion`

Generate shell completion scripts for bash, zsh, fish, or PowerShell.

```
minideploy completion <shell>
```

| Argument | Description |
|---|---|
| `bash` | Bash completions |
| `zsh` | Zsh completions |
| `fish` | Fish completions |
| `powershell` | PowerShell completions |

**Examples**:

```bash
# Bash — install permanently
minideploy completion bash | sudo tee /etc/bash_completion.d/minideploy

# Zsh — install permanently (create the directory if needed)
minideploy completion zsh | sudo tee /usr/local/share/zsh/site-functions/_minideploy

# Fish — install permanently
minideploy completion fish | sudo tee /etc/fish/completions/minideploy.fish

# Or source directly in your current shell
source <(minideploy completion bash)
```
