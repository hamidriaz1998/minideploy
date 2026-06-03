# Rollback & Destroy

## Rollback

Rollback reverts the `current` symlink to a previous release and restarts all service instances. It does NOT re-upload anything — it uses the immutable releases already on disk.

### How It Works

```
Before rollback:

  current ──► releases/20260603-143022/  (broken, buggy)

  releases/
    ├── 20260603-143022/    ← current (bad)
    ├── 20260602-120000/    ← most recent previous
    └── 20260601-090000/    ← old

After rollback (--release 20260602-120000):

  current ──► releases/20260602-120000/  (restored)

  releases/  (all dirs intact — can roll forward again)
```

### Finding the Previous Release

When you run `minideploy rollback` without specifying a release name, the daemon:

1. Reads the `current` symlink target → `releases/20260603-143022/`
2. Lists all directories in `releases/`
3. Filters out the current one
4. Picks the one with the most recent timestamp

If you want to rollback to a specific release, specify its name:

```bash
minideploy rollback 20260601-090000
```

### Client Usage

```bash
# Rollback to the previous release
minideploy rollback

# Rollback to a specific release
minideploy rollback 20260601-090000
```

### Daemon Behavior

```
POST /api/v1/rollback { "app_name": "my-api", "release_name": "..." }
  1. Lookup the app in state
  2. If no release_name specified, auto-detect previous
  3. Verify the release directory exists on disk
  4. Atomic symlink swap (same as deploy)
  5. Restart all instances (same as deploy)
  6. Update state.json with new current release
  7. Return response
```

Rollback is treated as a deployment — the previous release becomes the new "current" and is restarted. You can rollback repeatedly to any release.

---

## Destroy

Destroy removes an app from the daemon. Two modes: **soft** and **hard**.

### Soft Destroy

Stops all running service instances and unregisters the app from `state.json`. Leaves all files on disk (`/opt/<app>/upload`, `releases/`, `current`) for potential recovery.

Use when:
- You want to temporarily disable an app
- You need to re-register it later without re-uploading
- You're not sure if you'll need the releases again

```bash
minideploy destroy --soft --confirm
```

### Hard Destroy

Stops all service instances, removes the entire `deploy_path` directory (`/opt/<app>/`), and unregisters the app. Everything is gone.

Use when:
- You're decommissioning an app permanently
- You want to reclaim disk space
- You're starting fresh

```bash
minideploy destroy --confirm   # --soft omitted = hard destroy
```

### Safety

The `--confirm` flag (or `-y`) is mandatory. Without it, the daemon rejects the request:

```
$ minideploy destroy
error: --confirm is required to destroy an app

$ minideploy destroy --soft
error: --confirm is required to destroy an app
```

The daemon also validates the `confirm` field in the JSON body:

```json
// Request (rejected)
{ "app_name": "my-api", "soft": false, "confirm": false }
// → 400: confirmation required: set confirm=true
```

### What Gets Destroyed

| | Soft | Hard |
|---|---|---|
| Service instances stopped | Yes | Yes |
| App removed from state.json | Yes | Yes |
| `/opt/<app>/` directory | Kept | Removed |
| Releases on disk | Kept | Removed |
| systemd unit files | Untouched | Untouched |
| Daemon itself | Unaffected | Unaffected |

> **Note**: systemd unit files (e.g., `my-api@3000.service`) are managed by you and are never touched by minideploy, even during a hard destroy. You must remove them manually if desired.
