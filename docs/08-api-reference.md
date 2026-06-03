# API Reference

The daemon exposes a REST API on `http://127.0.0.1:8443/api/v1/`.

## Authentication

All endpoints except `/health` require a Bearer token:

```
Authorization: Bearer <api_key>
```

Invalid or missing tokens return `401 Unauthorized`.

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
  "deploy_path": "/opt/my-api"
}
```

All fields except `app_name` are optional:
- If omitted, `service_type`, `service_name`, `instances`, `deploy_path` use the previously stored values for this app
- If `release_name` is omitted, it's auto-generated as `YYYYMMDD-HHMMSS`

**Response**:

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

## Quick Reference

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/health` | No | Liveness check |
| `GET` | `/api/v1/status` | Yes | Daemon health |
| `GET` | `/api/v1/apps` | Yes | List apps |
| `GET` | `/api/v1/apps/:name` | Yes | App detail |
| `GET` | `/api/v1/apps/:name/status` | Yes | Per-instance status |
| `GET` | `/api/v1/apps/:name/releases` | Yes | Release history |
| `GET` | `/api/v1/apps/:name/logs` | Yes | App logs |
| `POST` | `/api/v1/deploy` | Yes | Trigger deploy |
| `POST` | `/api/v1/rollback` | Yes | Rollback |
| `POST` | `/api/v1/apps/:name/destroy` | Yes | Remove app |
