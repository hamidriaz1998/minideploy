# Overview & Architecture

## What is minideploy?

minideploy is a tool that automates the process of building, uploading, and deploying applications to a remote server. It consists of two components that share the same binary:

- **Client-side CLI** — runs on your development machine
- **Server-side Daemon** — runs on your VPS, exposing a REST API

## High-Level Flow

```
┌──────────────────────────────────────────────────────────────────┐
│                        Development Machine                        │
│                                                                    │
│  .deploy.yml                                                       │
│  ┌─────────────────────┐                                          │
│  │ app_name: my-api     │                                          │
│  │ build:               │  1. minideploy deploy                    │
│  │   - go build .      │  ┌──────────────────────┐                │
│  │ artifacts:           │  │ 1. Parse .deploy.yml │                │
│  │   - app              │  │ 2. Run build steps   │                │
│  │ server:              │  │ 3. rsync to server   │                │
│  │   host: my-vps       │  │ 4. POST /deploy      │                │
│  └─────────────────────┘  └──────────┬───────────┘                │
│                                      │                            │
└──────────────────────────────────────┼────────────────────────────┘
                                       │
              rsync over SSH ──────────┤
              POST /deploy ────────────┤
                                       │
┌──────────────────────────────────────┼────────────────────────────┐
│                                      ▼                            │
│                              VPS                                   │
│                                                                    │
│  ┌──────────────────────────────────────────────────────┐          │
│  │              minideploy Daemon (REST API)             │          │
│  │                                                       │          │
│  │  POST /deploy ──►                                    │          │
│  │  1. Copy upload/ → releases/20260603-143022/         │          │
│  │  2. Symlink swap: current → releases/<new>/           │          │
│  │  3. systemctl restart my-api@3000                     │          │
│  │  4. systemctl restart my-api@3001                     │          │
│  └──────────────────────────────────────────────────────┘          │
│                                                                    │
│  /opt/my-api/                                                      │
│  ├── upload/           ◄── rsync target (incremental)              │
│  ├── releases/                                                     │
│  │   ├── 20260603-143022/                                          │
│  │   └── 20260602-120000/                                          │
│  └── current ──► releases/20260603-143022/  (symlink)              │
└────────────────────────────────────────────────────────────────────┘
```

## Key Concepts

### Upload → Release Snapshot

The `upload/` directory is the only mutable directory. Every time you rsync, only changed files are transferred (rsync's delta algorithm). When you trigger a deploy, the daemon snapshots the current state of `upload/` into an immutable, timestamped release directory under `releases/`. This means:

- **Fast transfers** — unchanged files aren't re-uploaded
- **Immutable releases** — every deploy creates a complete snapshot that never changes
- **Instant rollback** — just re-point the `current` symlink

### Symlink-Based Zero-Downtime Deploys

The active release is always pointed to by a symlink at `<deploy_path>/current`. Your systemd service (or pm2) is configured to use this path. When deploying:

1. The new release is created in `releases/`
2. The symlink is atomically swapped (write to `.tmp`, then `rename`)
3. The service is restarted

Your app never sees a partially-written directory.

### Instance Pattern

For systemd, minideploy uses the `@` template syntax (`my-api@3000.service`). The `%i` placeholder in your `service_name` config is replaced with each instance's ID. This lets you run multiple copies of the same service on different ports with a single deploy command.

## Component Diagram

```
┌─────────────────────┐     ┌──────────────────────────────────────┐
│   Cobra CLI (cmd/)  │     │        Daemon (internal/daemon/)      │
│                     │     │                                        │
│  deploy/build       │     │  HTTP Server ──► Router ──► Auth MW   │
│  upload/rollback    │     │       │                                  │
│  status/ps/logs     │     │       ▼                                  │
│  init/init-server   │     │  Handlers                               │
│  destroy            │     │   ├── HandleDeploy -> DeployManager     │
└─────────┬───────────┘     │   │    ├── SnapshotRelease             │
          │                 │   │    ├── UpdateSymlink                │
          ▼                 │   │    └── ProcessManager.Restart       │
  ┌──────────────────┐      │   ├── HandleRollback                   │
  │  Client Libs      │      │   ├── HandleDestroy                   │
  │  (internal/client/)│     │   └── HandleStatus/Apps/...           │
  │                    │     │                                        │
  │  ConfigLoader      │     │  StateManager (state.json)            │
  │  BuildRunner       │     │  Auth (bcrypt API keys)               │
  │  RsyncRunner       │     │  ProcessManager (systemd | pm2)       │
  │  TunnelManager     │     └──────────────────────────────────────┘
  │  APIClient         │
  └──────────────────┘
```

## How the Deploy Pipeline Works

### Client Side (`minideploy deploy`)

```
  1. Parse .deploy.yml
  2. Execute build steps sequentially (sh -c each)
  3. Verify all artifacts exist on disk
  4. Resolve server.host:
       ├── valid IP → connect directly
       └── hostname → try DNS
                      ├── resolves → connect directly
                      └── fails → parse ~/.ssh/config for HostName
                                  └── start SSH tunnel
  5. rsync -avz --delete artifacts/ ssh_user@host:/opt/<app>/upload/
  6. POST /api/v1/deploy { app_name, release_name(optional), ... }
  7. Print result
```

### Server Side (Daemon — `POST /api/v1/deploy`)

```
  1. Validate API key (bcrypt compare)
  2. Register or update the app in state.json
  3. Generate release name (YYYYMMDD-HHMMSS) or use provided one
  4. Create /opt/<app>/releases/<release_name>/
  5. Copy all files from upload/ → release dir (cp -r semantics)
  6. Atomically swap symlink:
       a. os.Readlink("current") → save previous
       b. os.Symlink(releases/<new>, "current.tmp")
       c. os.Rename("current.tmp", "current")
  7. For each instance:
       a. Replace %i with instance ID in service_name
       b. sudo systemctl restart <unit>
       c. Verify status
  8. Persist release info to state.json
  9. Return response
```
