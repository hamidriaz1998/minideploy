# minideploy — Agent Instructions

## Build & Verify

```bash
go build ./... && go vet ./...
```

No tests exist yet (`go test ./...` returns nothing). No formatter config beyond default `gofmt`.

## Project Structure

Single binary (`main.go` → `cmd/` cobra root) with two modes:

- **`internal/client/`** — CLI-side logic: YAML config, build runner, rsync, SSH tunnel, HTTP API client
- **`internal/daemon/`** — Server daemon: HTTP server, REST handlers, SQLite state, process manager (systemd/pm2)
- **`internal/shared/`** — Types, logger, SSH config parser
- **`cmd/`** — One file per cobra subcommand (18 commands + root)

Key modules: `modernc.org/sqlite` (CGo-free SQLite), `charm.land/huh/v2` (interactive forms in `init`), `gopkg.in/yaml.v3`, `spf13/cobra`.

## Architecture

The daemon only listens on `127.0.0.1:8443` (localhost). To access remotely, the user manually runs `ssh -N -L 8443:127.0.0.1:8443 user@server` in a separate terminal. The `deploy` command auto-detects an existing tunnel via `IsPortOpen()` — if port 8443 is already listening locally, it skips creating a new tunnel.

## Rsync flags

Upload uses `-rlvz --delete --no-owner --no-group --no-perms --omit-dir-times`. No `-a` (archive) — `--no-*` flags explicitly disable ownership/metadata ops that fail against the minideploy-owned `upload/` directory.

