# sample-app

Minimal Go HTTP server for testing minideploy deployments.

## Quick Start

```bash
cd examples/sample-app
PORT=3000 go run .
```

## Endpoints

| Path | Description |
|------|-------------|
| `/` | Plain-text info (version, port) |
| `/health` | Health check JSON (`{"status":"ok"}`) |
| `/version` | Version info JSON |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | HTTP listen port |
| `VERSION` | `0.1.0` | Version string |

## Deploy with minideploy

1. Copy `.deploy.yml` to your project root and fill in your server details.
2. Run `minideploy deploy` from the project root.

The build step compiles the binary with an embedded git commit hash.
