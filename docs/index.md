# minideploy Documentation

minideploy is a single-binary server deployment manager. It builds your project, uploads artifacts via rsync, and deploys them through a server-side daemon with zero-downtime symlink swaps. Supports **systemd** and **pm2**.

## Getting Started

If you're new, start here:

1. [Installation & Setup](02-installation.md) — build the binary, bootstrap a VPS
2. [Configuration Guide](03-configuration.md) — write your first `.deploy.yml`
3. [Deployment Walkthrough](05-deployment.md) — deploy a real app end-to-end

## Reference

| Document | Description |
|---|---|
| [01 — Overview & Architecture](01-overview.md) | High-level design, components, data flow |
| [02 — Installation](02-installation.md) | Build, init-server, manual setup, upgrades |
| [03 — Configuration](03-configuration.md) | Full `.deploy.yml` reference |
| [04 — CLI Reference](04-cli-reference.md) | Every command, flag, and example |
| [05 — Deployment Walkthrough](05-deployment.md) | Step-by-step deploy with 3 example apps |
| [06 — Process Managers](06-process-managers.md) | systemd (@ instances), pm2, zero-downtime |
| [07 — Rollback & Destroy](07-rollback-destroy.md) | Rollback flow, soft vs hard destroy |
| [08 — API Reference](08-api-reference.md) | REST endpoints with request/response samples |
| [09 — Security](09-security.md) | Auth, privilege separation, sudoers, network |
| [10 — Server Layout](10-server-layout.md) | Disk layout, state.json, release naming |
| [11 — Troubleshooting](11-troubleshooting.md) | Common issues and solutions |
