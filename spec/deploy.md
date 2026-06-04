# Deploy Command Specification

## 1. Overview

The `deploy` command runs the full deployment pipeline: build the project locally, upload artifacts to a remote server, and trigger the minideploy daemon to snapshot, activate, and restart the service. It supports zero-downtime symlink swaps, health check verification, and automatic rollback on failure.

The deployment is split into two halves: **client-side** (developer's machine) and **daemon-side** (remote server).

---

## 2. CLI Interface

### 2.1 Usage

```
minideploy deploy [flags]
```

### 2.2 Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--config` | `-c` | string | `""` (auto-detect) | Path to `.deploy.yml` config file |
| `--release` | `-r` | string | `""` (auto-generated) | Custom release name |
| `--skip-build` | | bool | `false` | Skip build steps and artifact verification |
| `--skip-upload` | | bool | `false` | Skip rsync upload (deploy from existing `upload/` on server) |
| `--verbose` | `-v` | bool | `false` | Enable debug logging (global flag) |
| `--help` | | | | Show help |

`--skip-build` and `--skip-upload` are **independent**:
- `--skip-build` alone: upload still happens with previously built artifacts
- `--skip-upload` alone: build still runs (for local validation), but rsync is skipped
- Both together: no build, no upload — just call the daemon to deploy whatever is in `upload/`
- Neither (default): full pipeline

### 2.3 Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Deploy succeeded (all instances restarted, health checks passed) |
| `1` | Fatal error during any step (config, build, upload, tunnel, API) |

Note: Partial instance restart failures or health check rollbacks do **not** cause a non-zero exit — the CLI displays warnings but exits `0`. The deploy is considered "completed" even if degraded or rolled back.

---

## 3. YAML Configuration

### 3.1 Config File Discovery

The client searches for config files in the current working directory, in order:

1. `.deploy.yml`
2. `deploy.yml`
3. `.deploy.yaml`
4. `deploy.yaml`

The first match is used. The path can be overridden with `--config`.

### 3.2 Schema

| YAML Key | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `app_name` | string | yes | | Application name (identifier used in daemon state) |
| `service_type` | string | yes | | `systemd` or `pm2` |
| `service_name` | string | yes | | Service unit name; use `%i` as placeholder for instance ID |
| `instances` | array of Instance | no | `[]` | Per-instance config (id, port, env) |
| `deploy_path` | string | yes | | Absolute path on server (e.g. `/var/www/myapp`) |
| `build` | array of string | yes | | Shell commands to execute (at least one required) |
| `artifacts` | array of string | yes | | File/directory paths to rsync (at least one required) |
| `server.host` | string | yes | | Server hostname or IP |
| `server.api_port` | int | no | `8443` | Daemon HTTP API port |
| `server.ssh_user` | string | no | | SSH user for rsync and tunnel |
| `server.api_key` | string | see note | | API key (see resolution order below) |
| `keep_releases` | int | no | `10` | Number of old releases to retain (0 = keep all) |
| `health_check.endpoint` | string | no | `""` (no check) | URL path for health checks (e.g. `/health`) |
| `health_check.timeout` | int | conditional | | HTTP timeout in seconds (required if endpoint set) |
| `health_check.retries` | int | conditional | | Number of retries (required if endpoint set) |
| `health_check.wait_between_instances` | int | no | `0` | Seconds to wait between checking instances |
| `env` | map[string]string | no | | Environment variables (planned — see §11) |
| `pre_deploy` | array of Hook | no | | Pre-deploy hooks (planned — see §11) |
| `post_deploy` | array of Hook | no | | Post-deploy hooks (planned — see §11) |

### 3.3 Instance Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Instance identifier (e.g. `"1"`, `"blue"`) |
| `port` | int | yes | Port this instance listens on |
| `env` | map[string]string | no | Per-instance environment variables |

### 3.4 Hook Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `cmd` | string | yes | Shell command to execute on the server |

### 3.5 API Key Resolution Order

The daemon API key is resolved with the following precedence (first match wins):

1. `server.api_key` in YAML config
2. `MINIDEPLOY_API_KEY` environment variable
3. `.env` file in the current directory (looks for `MINIDEPLOY_API_KEY=<value>`)

If none is found, the CLI aborts with a fatal error before making any API call.

### 3.6 Validation Rules

- `app_name`, `service_type`, `service_name`, `deploy_path`, `server.host` must be non-empty
- `service_type` must be `"systemd"` or `"pm2"`
- `build` must have at least one entry
- `artifacts` must have at least one entry
- `keep_releases` cannot be negative; omitted or 0 means keep all
- If `health_check.endpoint` is non-empty, then `health_check.timeout` and `health_check.retries` must be positive; `health_check.wait_between_instances` cannot be negative
- `server.api_port` defaults to `8443` if omitted or 0

---

## 4. Client-Side Pipeline

Executed on the developer's machine in order.

### 4.1 Config Loading

1. Locate config file (auto-detect or `--config` flag)
2. Read and unmarshal YAML
3. Validate required fields and constraints
4. Resolve API key
5. Abort with fatal error on any failure

### 4.2 Build Execution

Skipped if `--skip-build` is set (regardless of `--skip-upload`).

1. For each entry in `build`, execute the command via `sh -c "<command>"`
2. Commands run sequentially, in the current working directory
3. If any command exits non-zero, abort with fatal error

### 4.3 Artifact Verification

Skipped if `--skip-build` is set (since artifacts come from the build).

1. For each path in `artifacts`, check that it exists on disk
2. If any artifact is missing, abort with fatal error

### 4.4 Rsync Upload

Skipped if `--skip-upload` is set.

1. Compute total artifact size for display
2. Run `rsync` with the following flags:

```
rsync -rlvz --delete --no-owner --no-group --no-perms --omit-dir-times <artifacts> <sshUser>@<host>:<deploy_path>/upload/
```

3. Destination directory on server: `{deploy_path}/upload/`
4. If rsync fails, abort with fatal error

Rsync flag rationale: `-a` (archive) is intentionally avoided. The `--no-*` flags disable ownership and metadata operations that would fail when rsync runs as the SSH user against the daemon-owned `upload/` directory.

### 4.5 SSH Tunnel Management

The daemon only listens on `127.0.0.1:8443`. The deploy command auto-detects whether an SSH tunnel is needed to reach it.

#### 4.5.1 Tunnel Decision Logic

```go
if NeedsTunnel(host) && !IsPortOpen("127.0.0.1", port) {
    // Case A: Host not DNS-resolvable, no local daemon → start tunnel
} else if NeedsTunnel(host) {
    // Case B: Host not DNS-resolvable, but port 8443 already open → reuse
} else {
    // Case C: Host is IP or DNS-resolvable → connect directly
}
```

`NeedsTunnel` returns `true` when:
- The host is **not** a raw IP address (all digits and `.`)
- AND `net.LookupHost(host)` fails (hostname does not resolve in DNS)

This covers hosts reachable only via SSH (e.g., internal hostnames, SSH config aliases).

`IsPortOpen` does a TCP dial to `127.0.0.1:<port>` with a 2-second timeout.

#### 4.5.2 Tunnel Lifecycle

1. If a new tunnel is needed, run: `ssh -N -L <localPort>:127.0.0.1:<remotePort> [sshUser@]host`
2. Detach the SSH process (no stdin, stderr to terminal)
3. Wait 500ms, check if the process exited immediately
4. If tunnel fails to start, abort with fatal error
5. Tunnel is killed (`defer tunnel.Close()`) when the deploy command completes

#### 4.5.3 Connection Destination

- Cases A/B (tunnel): API calls go to `127.0.0.1:<port>`
- Case C (direct): API calls go to `<host>:<port>`

### 4.6 Daemon API Call

1. Build `DeployRequest` from config (app_name, service_type, service_name, instances, deploy_path, keep_releases, health_check)
2. If `--release` was provided, set `ReleaseName`
3. Send `POST /api/v1/deploy` with `Authorization: Bearer <apiKey>`
4. HTTP client timeout: 60 seconds
5. Display result to user: release name, restarted instances, health check results, rollback warning

---

## 5. Daemon-Side Pipeline

Executed on the server inside the `POST /api/v1/deploy` handler. All steps run synchronously within a single HTTP request.

### 5.1 Request Validation

1. Decode JSON body into `DeployRequest`
2. Validate `app_name` is non-empty (400 Bad Request if missing)
3. Authorize the API key against the requested app (403 Forbidden if unauthorized)

### 5.2 Release Name Generation

If `ReleaseName` is empty, generate one from the current UTC time:

```
YYYYMMDD-HHMMSS
```

For custom release names, see §8.1 for validation rules.

### 5.3 App Registration / Update

Register the app in SQLite if it doesn't exist. If it does exist, update its `service_type`, `service_name`, `deploy_path`, and `instances` in-place. Both operations happen in a single transaction.

### 5.4 Directory Setup

Ensure the following directory structure exists on disk:

```
{deploy_path}/
├── upload/     (mode 0777 — world-writable for rsync)
└── releases/   (mode 0755)
```

The `upload/` directory is set to `0777` so the SSH user (who may differ from the daemon user) can write files via rsync.

### 5.5 Snapshot

Copy all files from `{deploy_path}/upload/` to `{deploy_path}/releases/{releaseName}/`. This is a file-by-file copy using `os.CopyFS`-style directory walking — not a symlink or hardlink.

The `upload/` directory is **not** cleared after snapshot; subsequent deploys without `--skip-upload` will rsync over it.

### 5.6 Atomic Symlink Swap

1. Remove `{deploy_path}/current.tmp` if it exists
2. Create symlink: `current.tmp → releases/{releaseName}`
3. Rename `current.tmp → current` (atomic on Linux — same filesystem)

This avoids the window where `current` doesn't exist. The **previous** target (before the swap) is captured for potential rollback.

### 5.7 Release Pruning

If `keep_releases > 0`:

1. Query all releases for this app from SQLite, ordered by creation time
2. Delete the oldest releases that exceed the keep count:
   - Remove release directory from disk
   - Delete release record from SQLite
3. Pruning errors are logged but **do not abort** the deploy

If `keep_releases` is `0`, no pruning occurs (keep all releases indefinitely).

### 5.8 Instance Restart

1. Create a `ProcessManager` based on `ServiceType` (`systemd` or `pm2`)
2. For each configured instance, build the unit name by replacing `%i` in `ServiceName` with the instance's `ID`
3. Restart each instance with a 30-second timeout
4. Collect lists of successfully restarted and failed instances
5. Continue even if some instances fail (do not abort the deploy)

**Process Manager Interface:**

| Service Type | Restart Mechanism |
|-------------|-------------------|
| `systemd` | `systemctl restart <unit>` |
| `pm2` | `pm2 restart <name>` |

### 5.9 Health Checks

Only runs if **all** of the following are true:
- All instances restarted successfully
- `HealthCheck.Endpoint` is non-empty
- At least one instance is configured

For each instance:

1. Build URL: `http://localhost:{instance.port}{endpoint}`
2. Retry loop: up to `retries` attempts with 2-second intervals
3. Consider 2xx or 3xx HTTP status as passing
4. Wait `wait_between_instances` seconds before checking the next instance

### 5.10 Automatic Rollback

If any health check fails:

1. Restore the symlink: `current → {previous release}` (using the same atomic swap)
2. Restart all instances with the old release
3. Update SQLite: set the previous release as `is_current = 1`
4. Return response with `rolled_back: true` and `rolled_back_to: "<previous release>"`

Rollback restart errors are silently ignored (the old release files are in place even if the process fails to start).

### 5.11 State Persistence

If NOT rolled back:

1. Set all existing releases for this app to `is_current = 0`
2. Insert the new release with `is_current = 1`
3. If the DB insert fails, the deploy is still considered successful (the symlink swap already happened)

---

## 6. Deploy Lifecycle State Machine

Each deploy transitions through explicit states on the daemon side. These states are **not** persisted as a separate field; they are implicit in the pipeline's progress. The states below define the contract for testing and observability.

```
                  ┌──────────┐
                  │ PENDING  │
                  └────┬─────┘
                       │
                  ┌────▼──────┐
                  │ SNAPSHOT  │
                  └────┬──────┘
                       │
                  ┌────▼───────┐
                  │ ACTIVATING │  (symlink swap)
                  └────┬───────┘
                       │
                  ┌────▼─────────┐
                  │ RESTARTING   │
                  └────┬─────────┘
                       │
              ┌────────┴────────┐
              │                 │
     ┌────────▼───────┐  ┌─────▼──────────┐
     │ HEALTH_CHECK   │  │   DEGRADED     │
     │ (if configured)│  │ (some instances│
     └────────┬───────┘  │  failed)       │
              │          └────────────────┘
       ┌──────┴──────┐
       │             │
  ┌────▼───┐   ┌────▼──────┐
  │  LIVE  │   │ ROLLED_BK │
  └────────┘   └───────────┘

  Any step can transition to:  FAILED  (unrecoverable error)
```

### 6.1 State Descriptions

| State | Meaning |
|-------|---------|
| `PENDING` | Release registered in DB, files not yet snapshotted |
| `SNAPSHOT` | Files being copied from `upload/` to `releases/{name}/` |
| `ACTIVATING` | Atomic symlink swap in progress |
| `RESTARTING` | Instances being restarted via systemd/pm2 |
| `HEALTH_CHECK` | Health checks running against instances |
| `LIVE` | Deploy succeeded, all instances healthy |
| `DEGRADED` | Deploy completed but some instances failed to restart |
| `ROLLED_BACK` | Health check failed, symlink reverted to previous release |
| `FAILED` | Unrecoverable error (config, disk, DB, etc.) |

### 6.2 Valid Transitions

| From | To | Condition |
|------|----|-----------|
| `PENDING` | `SNAPSHOT` | Snapshot begins |
| `SNAPSHOT` | `ACTIVATING` | Snapshot completes |
| `SNAPSHOT` | `FAILED` | Snapshot copy error |
| `ACTIVATING` | `RESTARTING` | Symlink swap succeeds |
| `ACTIVATING` | `FAILED` | Symlink/rename error |
| `RESTARTING` | `HEALTH_CHECK` | All instances restarted and health check configured |
| `RESTARTING` | `DEGRADED` | Some instances failed (health check skipped) |
| `RESTARTING` | `LIVE` | No instances configured (nothing to restart) |
| `HEALTH_CHECK` | `LIVE` | All health checks pass |
| `HEALTH_CHECK` | `ROLLED_BACK` | Any health check fails (and previous release exists) |
| `HEALTH_CHECK` | `DEGRADED` | Any health check fails AND no previous release to roll back to |
| `ROLLED_BACK` | `FAILED` | Rollback restart fails |
| `DEGRADED` | (terminal) | |
| `LIVE` | (terminal) | |
| `ROLLED_BACK` | (terminal) | |
| `FAILED` | (terminal) | |

---

## 7. HTTP API Contract

### 7.1 Endpoint

```
POST /api/v1/deploy
Content-Type: application/json
Authorization: Bearer <apiKey>
```

### 7.2 Request Body

```json
{
  "app_name": "myapp",
  "release_name": "20250101-120000",
  "service_type": "systemd",
  "service_name": "myapp-%i",
  "instances": [
    { "id": "1", "port": 8080, "env": { "NODE_ENV": "production" } }
  ],
  "deploy_path": "/var/www/myapp",
  "keep_releases": 10,
  "health_check": {
    "endpoint": "/health",
    "timeout": 5,
    "retries": 3,
    "wait_between_instances": 2
  }
}
```

Fields `app_name`, `service_type`, `service_name`, `deploy_path` are only used for initial registration or update. On subsequent deploys, the daemon may merge with stored state.

### 7.3 Response Body (Success)

```json
{
  "success": true,
  "data": {
    "release": "20250101-120000",
    "instances": ["myapp-1"],
    "app_name": "myapp",
    "health_results": [
      { "instance": "myapp-1", "port": 8080, "passed": true, "error": "" }
    ],
    "rolled_back": false,
    "rolled_back_to": ""
  },
  "error": ""
}
```

### 7.4 Response Body (Rolled Back)

```json
{
  "success": true,
  "data": {
    "release": "20250101-120000",
    "instances": ["myapp-1"],
    "app_name": "myapp",
    "health_results": [
      { "instance": "myapp-1", "port": 8080, "passed": false, "error": "connection refused" }
    ],
    "rolled_back": true,
    "rolled_back_to": "20250101-110000"
  },
  "error": ""
}
```

### 7.5 Response Body (Error)

```json
{
  "success": false,
  "data": null,
  "error": "app_name is required"
}
```

### 7.6 HTTP Status Codes

| Code | Meaning |
|------|---------|
| `200` | Deploy completed (may be live, degraded, or rolled back) |
| `400` | Invalid request body or missing required field |
| `403` | API key not authorized for this app |
| `500` | Internal error (disk, DB, process manager) |

The `success` field in the JSON body is distinct from the HTTP status code. All completed deploys (even rolled back or degraded) return HTTP 200 with `success: true`. Only unrecoverable errors return HTTP 4xx/5xx.

### 7.7 Auth

The daemon authenticates requests via `Bearer` tokens. API keys are stored as bcrypt hashes in SQLite with two scopes:

| Scope | Description |
|-------|-------------|
| `global` | Can access all apps |
| `app` | Restricted to a specific `app_name` |

The auth middleware extracts the token, validates it against all stored hashes, and injects the scope and app name into the request context.

---

## 8. Release Management

### 8.1 Naming

**Auto-generated:** `YYYYMMDD-HHMMSS` (UTC). Example: `20250101-120000`.

**Custom names** (via `--release` flag): must pass the following validation:

- Maximum length: **64 characters**
- Allowed characters: `[a-zA-Z0-9._-]`
- Must not contain: `..`, `/`, `\`, null bytes (path traversal prevention)

### 8.2 Retention & Pruning

- Controlled by `keep_releases` in config (default: `10`, `0` = keep all)
- Pruning runs after successful deploy (post-symlink, post-restart)
- Oldest releases beyond the keep count are pruned — both release directory on disk and DB records
- The `current` release is never pruned (DB won't have `is_current=1` for it, but in practice the most recent releases are the newest, so old ones are pruned first)

---

## 9. Edge Cases & Error Handling

### 9.1 Partial Instance Restart Failure

If one or more instances fail to restart:
- The deploy is **not** rolled back
- The response sets `success: false` and lists failed instances
- Health checks are **skipped** entirely (cannot health-check a failed instance)
- State: `DEGRADED`
- The CLI warns the user but exits `0`

### 9.2 Health Check Failure

If all instances restarted but health checks fail:
- The daemon performs an **automatic rollback** (revert symlink, restart with old release)
- Response includes `rolled_back: true` and `rolled_back_to: <release>`
- State: `ROLLED_BACK`
- If no previous release exists for rollback, the new release stays active despite health failures (state: `DEGRADED`)

### 9.3 Concurrent Deploys

Multiple deploys for the same app **must not** be run concurrently — there is no locking or queue. Doing so would cause races on:
- The symlink swap
- The snapshot (upload/ being read while being written)
- The DB release records

The spec assumes callers serialize deploys per app. No protection is provided by the daemon.

### 9.4 Rsync Permission Model

- `upload/` is created with mode `0777` so the SSH user can write regardless of the daemon user
- Rsync flags avoid `-a` and explicitly skip ownership/permissions/times to work across different users
- Artifacts are copied into `upload/`; the daemon reads them via snapshot (not directly from upload/)

### 9.5 Snapshot Race

Between the rsync completing and the snapshot starting, there is a window where new files could appear in `upload/`. The daemon snapshots whatever is present at snapshot time. This is considered acceptable; the `upload/` directory is not locked during the window.

### 9.6 Disk Space

No disk space check is performed before snapshot or pruning. If the disk is full, the snapshot or copy will fail and the deploy will abort with a `FAILED` state.

### 9.7 Daemon Restart During Deploy

The deploy runs in a single HTTP request. If the daemon restarts mid-deploy:
- The HTTP request is interrupted
- The client sees a connection error
- The server state may be partially updated (app registered but no release persisted)
- Manual inspection via `minideploy status` or `minideploy releases` is required

---

## 10. Security Model

### 10.1 API Keys

- Stored as bcrypt hashes in SQLite
- Two scopes: `global` and `app`-scoped
- Scoped keys can only deploy/rollback/status their specific app
- Global keys can manage all apps and perform admin actions (key management, app destroy)

### 10.2 Network

- The daemon binds **only** to `127.0.0.1:8443` (localhost)
- Remote access requires an SSH tunnel (managed by the client or manually)

### 10.3 Filesystem

- The daemon only writes to its configured `deploy_path` and state directory
- Release names are validated against path traversal (see §8.1)

---

## 11. Planned Features

These are defined in the YAML schema but not yet implemented.

### 11.1 Pre/Post Deploy Hooks

**Timing:**
- `pre_deploy`: runs on the **server** after the snapshot (`§5.5`) but before the symlink swap (`§5.6`)
- `post_deploy`: runs on the **server** after health checks pass (`§5.9`) but before the response is sent (`§5.11`)

**Use cases:**
- `pre_deploy`: database migrations, cache warming, config validation
- `post_deploy`: notification (Slack, webhook), metrics recording, log shipping

Each hook is a shell command executed on the server. If a hook fails, the deploy should abort/fail.

### 11.2 Environment Variable Injection

The `env` block in the YAML config is reserved for injecting environment variables into build steps. When implemented:
- Each build command should inherit the environment variables from `env`
- This eliminates the need for `.env` files or shell `export` in build steps
