# Process Managers

minideploy abstracts process management behind a common interface. Currently supports **systemd** and **pm2**. The process manager is selected per app via `service_type` in `.deploy.yml`.

## The ProcessManager Interface

```go
type ProcessManager interface {
    Restart(ctx, serviceName, instanceID) error
    Start(ctx, serviceName, instanceID) error
    Stop(ctx, serviceName, instanceID) error
    Status(ctx, serviceName, instanceID) (ProcessStatus, error)
    Logs(ctx, serviceName, instanceID, lines int) (string, error)
}
```

Both implementations translate these calls into the appropriate system commands:

| Action | systemd | pm2 |
|---|---|---|
| Restart | `sudo systemctl restart <unit>` | `sudo pm2 restart <name>` |
| Start | `sudo systemctl start <unit>` | `sudo pm2 start <name>` |
| Stop | `sudo systemctl stop <unit>` | `sudo pm2 stop <name>` |
| Status | `sudo systemctl show <unit>` | `sudo pm2 jlist` |
| Logs | `sudo journalctl -u <unit> -n <lines>` | `sudo pm2 logs <name> --lines <lines>` |

---

## systemd

### Template Units with `@`

systemd supports [template units](https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html#Service%20Templates) using the `@` syntax. A file named `my-api@.service` becomes a template that can be instantiated as `my-api@3000.service`, `my-api@3001.service`, etc.

In your `.deploy.yml`:

```yaml
service_name: my-api@%i
instances:
  - id: "3000"
    port: 3000
  - id: "3001"
    port: 3001
```

minideploy replaces `%i` with each instance's `id` field. When it restarts services, it runs:

```
sudo systemctl restart my-api@3000.service
sudo systemctl restart my-api@3001.service
```

### Example Template Unit

`/etc/systemd/system/my-api@.service`:

```ini
[Unit]
Description=My API (instance %i)
After=network.target

[Service]
Type=simple
User=my-api
ExecStart=/opt/my-api/current/app
Restart=always
RestartSec=3
EnvironmentFile=/opt/my-api/env-%i.conf

[Install]
WantedBy=multi-user.target
```

Key points:
- The unit file name ends with `@.service` — this makes it a template
- `%i` in the `Description` and `EnvironmentFile` paths is replaced with the instance ID
- `ExecStart` points to `/opt/my-api/current/app` — a symlink managed by minideploy
- Each instance gets its own environment file with its port

### Environment Files

Create environment files for each instance:

`/opt/my-api/env-3000.conf`:
```
PORT=3000
NODE_ENV=production
```

`/opt/my-api/env-3001.conf`:
```
PORT=3001
NODE_ENV=production
```

### Enabling Instances

```bash
sudo systemctl enable my-api@3000
sudo systemctl enable my-api@3001
sudo systemctl start my-api@3000
sudo systemctl start my-api@3001
```

### Checking Status

```bash
$ systemctl status my-api@3000
● my-api@3000.service - My API (instance 3000)
     Loaded: loaded (/etc/systemd/system/my-api@.service; enabled)
     Active: active (running)
   Main PID: 12345
      Tasks: 6
     Memory: 15.2M
```

---

## pm2

### Configuration

In your `.deploy.yml`:

```yaml
service_type: pm2
service_name: my-api-%i
instances:
  - id: "3000"
    port: 3000
  - id: "3001"
    port: 3001
```

minideploy runs:
```
sudo pm2 restart my-api-3000
sudo pm2 restart my-api-3001
```

### Managing the App

First, start the app on the server manually (or via a one-shot setup):

```bash
pm2 start /opt/my-api/current/server.js --name my-api-3000
pm2 start /opt/my-api/current/server.js --name my-api-3001
pm2 save
```

After that, `minideploy deploy` handles restarts automatically.

### pm2 Status

```bash
$ pm2 list
┌─────┬───────────────┬──────────┬──────┬───────────┐
│ id  │ name          │ mode     │ ↺    │ status    │
├─────┼───────────────┼──────────┼──────┼───────────┤
│ 0   │ my-api-3000   │ fork     │ 0    │ online    │
│ 1   │ my-api-3001   │ fork     │ 1    │ online    │
└─────┴───────────────┴──────────┴──────┴───────────┘
```

---

## Upload → Release Snapshot Concept

This is a core design decision that deserves explanation.

### The Problem

When deploying, you want:
1. **Fast uploads** — don't re-send files that haven't changed
2. **Immutable releases** — every deploy is a complete snapshot
3. **Instant rollback** — revert to any previous state

### The Solution: Two-Stage Pipeline

```
  rsync ──────►  upload/  (mutable, incremental)
                      │
                      ▼  (daemon snapshots on deploy)
              releases/20260603-143022/  (immutable, complete copy)
                      │
                      ▼  (symlink swap)
              current ──► releases/20260603-143022/
```

### Stage 1: `upload/` (Mutable)

- rsync target: `rsync -avz --delete artifacts/ ssh_user@host:/opt/<app>/upload/`
- Files accumulate here across deploys
- rsync's delta algorithm only transfers changed bytes
- Unchanged files (like `node_modules/`) are never re-uploaded
- This directory is **never** served directly — it's just a staging area

### Stage 2: `releases/<name>/` (Immutable)

- Created by the daemon when you call `POST /deploy`
- Complete copy of `upload/` at that point in time
- Named by timestamp: `YYYYMMDD-HHMMSS`
- Never modified after creation
- Each release is a full, self-contained snapshot

### Stage 3: `current` (Symlink)

- Atomic symlink pointing to the active release
- Updated via: `os.Symlink → os.Rename` (atomic on Linux)
- Your systemd service or pm2 config points to `current/`
- The service never sees a half-written directory

### Why Not Just rsync Directly to the Release?

If you rsync'd directly to `releases/20260603-143022/`, every deploy would re-upload everything — rsync has no previous state to diff against. The `upload/` directory preserves the previous state so rsync's delta algorithm works.

### Why Make Copies Instead of Using upload/ Directly?

If you served directly from `upload/`, a partial or failed rsync would leave the app in a broken state. By snapshotting to an immutable release, you guarantee that:

1. The active release is always complete
2. Rollback is just a symlink change (instant, no file operations)
3. You can audit exactly what was deployed and when
