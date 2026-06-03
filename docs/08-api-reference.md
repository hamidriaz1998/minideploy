# API Reference

The daemon exposes a REST API on `http://127.0.0.1:8443/api/v1/`.

## Authentication

All endpoints except `/health` require a Bearer token:

```
Authorization: Bearer <api_key>
```

Invalid or missing tokens return `401 Unauthorized`.

### Key Scoping

minideploy supports two types of API keys:

- **Global keys** — Full access to all endpoints and all apps
- **App-scoped keys** — Limited to `deploy`, `rollback`, `status`, `releases`, `logs` for a single app

The following endpoints require a **global-scoped** key (app-scoped keys are rejected):

| Endpoint | Reason |
|---|---|
| `POST /api/v1/rotate-key` | Key management |
| `POST /api/v1/keys` | Key management |
| `GET /api/v1/keys` | Key management |
| `DELETE /api/v1/keys/:id` | Key management |
| `POST /api/v1/apps/:name/destroy` | App destruction |

App-scoped keys calling `GET /api/v1/apps` will see **only their own app** in the response.

## Response Format

All responses use a uniform envelope:

```json
{
  "success": true,
  "data": { ... },
  "error": null
}
```

On failure:

```json
{
  "success": false,
  "data": null,
  "error": "description of what went wrong"
}
```

---

## `GET /api/v1/health`

Public endpoint. No auth required. Quick liveness check.

**Response**:

```json
{
  "success": true,
  "data": {
    "status": "ok"
  }
}
```

**Status codes**: `200`

---

## `GET /api/v1/status`

Daemon health information.

**Response**:

```json
{
  "success": true,
  "data": {
    "version": "0.1.0",
    "uptime": "3h12m45s",
    "start_time": "2026-06-03T12:00:00Z",
    "apps_count": 2,
    "disk_usage": {
      "total": 102400000000,
      "used": 51200000000,
      "available": 51200000000
    }
  }
}
```

**Status codes**: `200`

---

## `GET /api/v1/apps`

List all registered apps.

**Response**:

```json
{
  "success": true,
  "data": [
    {
      "name": "my-api",
      "service_type": "systemd",
      "current_release": "20260603-143022",
      "instances_count": 2,
      "running": true
    }
  ]
}
```

**Status codes**: `200`

---

## `GET /api/v1/apps/:name`

Get detailed information about a specific app.

**Response**:

```json
{
  "success": true,
  "data": {
    "name": "my-api",
    "service_type": "systemd",
    "service_name": "my-api@%i",
    "deploy_path": "/opt/my-api",
    "instances": [
      { "id": "3000", "port": 3000, "env": { "PORT": "3000" } },
      { "id": "3001", "port": 3001, "env": { "PORT": "3001" } }
    ],
    "current_release": "20260603-143022",
    "releases": [
      { "name": "20260603-143022", "created_at": "2026-06-03T14:30:22Z", "is_current": true },
      { "name": "20260602-120000", "created_at": "2026-06-02T12:00:00Z", "is_current": false }
    ]
  }
}
```

**Status codes**: `200`, `404`

---

## `GET /api/v1/apps/:name/status`

Get per-instance process status for an app.

**Response**:

```json
{
  "success": true,
  "data": {
    "app_name": "my-api",
    "current_release": "20260603-143022",
    "instances": [
      { "id": "3000", "port": 3000, "running": true, "pid": 12345 },
      { "id": "3001", "port": 3001, "running": true, "pid": 12346 }
    ]
  }
}
```

**Status codes**: `200`, `404`

---

## `GET /api/v1/apps/:name/releases`

List all releases for an app.

**Response**:

```json
{
  "success": true,
  "data": [
    { "name": "20260603-143022", "created_at": "2026-06-03T14:30:22Z", "is_current": true },
    { "name": "20260602-120000", "created_at": "2026-06-02T12:00:00Z", "is_current": false },
    { "name": "20260601-090000", "created_at": "2026-06-01T09:00:00Z", "is_current": false }
  ]
}
```

**Status codes**: `200`, `404`

---

## `GET /api/v1/apps/:name/logs`

Fetch the latest log entries for all instances of an app.

**Response**: Plain text with instance headers

```
--- my-api@3000 ---
Jun 03 14:30:22 vps systemd[1]: Started my-api@3000.service
Jun 03 14:30:22 vps my-api[12345]: Server listening on port 3000

--- my-api@3001 ---
Jun 03 14:30:22 vps systemd[1]: Started my-api@3001.service
Jun 03 14:30:22 vps my-api[12346]: Server listening on port 3001
```

**Content-Type**: `text/plain`

**Status codes**: `200`, `404`

---

## `POST /api/v1/deploy`

Trigger a deploy. Copies `upload/` → a new release, swaps the symlink, and restarts all instances.

**Request**:

```json
{
  "app_name": "my-api",
  "release_name": "20260603-143022",
  "service_type": "systemd",
  "service_name": "my-api@%i",
  "instances": [
    { "id": "3000", "port": 3000, "env": { "PORT": "3000" } }
  ],
  "deploy_path": "/opt/my-api",
  "keep_releases": 5,
  "health_check": {
    "endpoint": "/health",
    "timeout": 10,
    "retries": 3,
    "wait_between_instances": 1
  }
}
```

All fields except `app_name` are optional:
- If omitted, `service_type`, `service_name`, `instances`, `deploy_path` use the previously stored values for this app
- If `release_name` is omitted, it's auto-generated as `YYYYMMDD-HHMMSS`
- `keep_releases` controls pruning of old releases (0 = keep all)
- If `health_check` is configured and all instances restart successfully, the daemon will HTTP GET each instance at `http://localhost:<port><endpoint>` with the configured retries. If any instance fails its health check, the deploy is **automatically rolled back** to the previous release.

**Response** (success, no health check):

```json
{
  "success": true,
  "data": {
    "release": "20260603-143022",
    "instances": ["my-api@3000", "my-api@3001"],
    "app_name": "my-api"
  },
  "error": null
}
```

**Response** (successful deploy with health checks):

```json
{
  "success": true,
  "data": {
    "release": "20260603-143022",
    "instances": ["my-api@3000", "my-api@3001"],
    "app_name": "my-api",
    "health_results": [
      { "instance": "3000", "port": 3000, "passed": true },
      { "instance": "3001", "port": 3001, "passed": true }
    ]
  },
  "error": null
}
```

**Response** (health check failed, auto-rolled back):

```json
{
  "success": true,
  "data": {
    "release": "20260603-143022",
    "instances": ["my-api@3000", "my-api@3001"],
    "app_name": "my-api",
    "health_results": [
      { "instance": "3000", "port": 3000, "passed": true },
      { "instance": "3001", "port": 3001, "passed": false, "error": "status 503" }
    ],
    "rolled_back": true,
    "rolled_back_to": "20260602-120000"
  },
  "error": null
}
```

If some instances fail to restart, the response still returns `200` with `success: false` and the error field listing which instances failed:

```json
{
  "success": false,
  "data": {
    "release": "20260603-143022",
    "instances": ["my-api@3000"],
    "app_name": "my-api"
  },
  "error": "some instances failed: my-api@3001"
}
```

**Status codes**: `200` (even if some instances fail), `400`, `500`

---

## `POST /api/v1/rollback`

Rollback to a previous release. The symlink is re-pointed and all instances are restarted.

**Request**:

```json
{
  "app_name": "my-api",
  "release_name": "20260602-120000"
}
```

If `release_name` is omitted, the daemon auto-detects the previous release (the most recent directory in `releases/` that isn't the current one).

**Response**:

```json
{
  "success": true,
  "data": {
    "release": "20260602-120000",
    "instances": ["my-api@3000", "my-api@3001"]
  }
}
```

**Status codes**: `200`, `400`, `404`, `500`

---

## `POST /api/v1/apps/:name/destroy`

Remove an app from the daemon. Requires `"confirm": true`.

**Request**:

```json
{
  "app_name": "my-api",
  "soft": false,
  "confirm": true
}
```

| Field | Default | Description |
|---|---|---|
| `soft` | `false` | Keep files on disk (soft) or remove everything (hard) |
| `confirm` | `false` | Must be `true` to proceed |

**Response**:

```json
{
  "success": true,
  "data": {
    "app_name": "my-api",
    "soft": false
  }
}
```

**Status codes**: `200`, `400` (missing confirm), `404`, `500`

---

---

## `POST /api/v1/rotate-key`

Generate a new API key. Optionally revoke all previous keys.

**Request**:

```json
{
  "revoke_old": false
}
```

| Field | Default | Description |
|---|---|---|
| `revoke_old` | `false` | If true, all previous keys are invalidated |

**Response**:

```json
{
  "success": true,
  "data": {
    "new_key": "a1b2c3d4e5f6...",
    "keys_count": 2
  }
}
```

The raw key is returned **only in this response** and cannot be retrieved later. Save it immediately.

**Requires**: Global-scoped key.

**Status codes**: `200`, `403` (app-scoped key), `500`

---

## `POST /api/v1/keys`

Create a new API key with a specific scope and optional app name and label.

**Requires**: Global-scoped key.

**Request**:

```json
{
  "scope": "app",
  "app_name": "my-api",
  "label": "CI/CD deploy key"
}
```

| Field | Default | Description |
|---|---|---|
| `scope` | `app` | `"global"` or `"app"` |
| `app_name` | `""` | Required when `scope` is `"app"` |
| `label` | `""` | Optional human-readable label |

**Response**:

```json
{
  "success": true,
  "data": {
    "id": 2,
    "raw_key": "b2c3d4e5f6a7...",
    "scope": "app",
    "app_name": "my-api",
    "label": "CI/CD deploy key"
  }
}
```

The raw key is returned **only in this response** and cannot be retrieved later. Save it immediately.

**Status codes**: `201`, `400`, `403` (app-scoped key), `500`

---

## `GET /api/v1/keys`

List all API keys registered with the daemon.

**Requires**: Global-scoped key.

**Response**:

```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "scope": "global",
      "app_name": "",
      "label": "initial key",
      "hash_hint": "$2a$10$abc...",
      "created_at": "2026-06-03T12:00:00Z"
    }
  ]
}
```

**Status codes**: `200`, `403` (app-scoped key)

---

## `DELETE /api/v1/keys/:id`

Permanently delete an API key by its database ID.

**Requires**: Global-scoped key.

**Request**: No body. The key ID is in the URL path.

**Response**:

```json
{
  "success": true,
  "data": {
    "deleted": true
  }
}
```

**Status codes**: `200`, `404` (key not found), `403` (app-scoped key)

---

## Quick Reference

| Method | Path | Auth | Scope | Description |
|---|---|---|---|---|---|
| `GET` | `/api/v1/health` | No | — | Liveness check |
| `GET` | `/api/v1/status` | Yes | Any | Daemon health |
| `GET` | `/api/v1/apps` | Yes | Any | List apps (filtered for app-scoped keys) |
| `GET` | `/api/v1/apps/:name` | Yes | Any | App detail |
| `GET` | `/api/v1/apps/:name/status` | Yes | Any | Per-instance status |
| `GET` | `/api/v1/apps/:name/releases` | Yes | Any | Release history |
| `GET` | `/api/v1/apps/:name/logs` | Yes | Any | App logs |
| `POST` | `/api/v1/deploy` | Yes | Any | Trigger deploy |
| `POST` | `/api/v1/rollback` | Yes | Any | Rollback |
| `POST` | `/api/v1/rotate-key` | Yes | Global | Generate new API key |
| `POST` | `/api/v1/keys` | Yes | Global | Create API key |
| `DELETE` | `/api/v1/keys/:id` | Yes | Global | Delete API key |
| `GET` | `/api/v1/keys` | Yes | Global | List API keys |
| `POST` | `/api/v1/apps/:name/destroy` | Yes | Global | Remove app |
