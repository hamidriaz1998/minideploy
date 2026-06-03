# Security

## Authentication

### API Key Model

minideploy uses a pre-shared API key for all daemon communication (except the `/health` endpoint).

1. **Generation**: `init-server` (or the daemon on first start) generates a 64-character hex string from `crypto/rand`
2. **Storage**: The key is bcrypt-hashed and stored in `/var/lib/minideploy/state.json`
3. **Transmission**: The client sends it as a `Bearer` token in the `Authorization` header
4. **Verification**: The daemon uses `bcrypt.CompareHashAndPassword` on every request

### Key Distribution

```
init-server                          User
    │                                  │
    ├── Generate key ──────────────────┤
    ├── Store bcrypt hash              │
    └── Print raw key ────────────────►│
                                       ├── Save to .deploy.yml
                                       ├── Save to .env (gitignored)
                                       └── Or export as env var
```

The raw key is printed **once** during setup. If lost, you can:

- Check daemon logs on first start: `journalctl -u minideploy | grep "Generated one-time key"`
- Generate a new key (future feature)

### Key Rotation

To rotate the API key (future: `minideploy rotate-key`), you can manually:

1. Generate a new key: `openssl rand -hex 32`
2. Hash it: (requires manual bcrypt)
3. Edit `/var/lib/minideploy/state.json` to add the new hash
4. Remove the old hash after updating all clients

---

## Privilege Separation

### User Model

```
┌─────────────────────────────────────────────────────┐
│                  System Users                         │
│                                                       │
│  root                                                │
│    ├── Installs daemon (init-server)                 │
│    └── Manages systemd unit files                    │
│                                                       │
│  minideploy (system user, no shell)                  │
│    ├── Runs the daemon process                       │
│    ├── Owns /opt/<app>/ and /var/lib/minideploy/     │
│    └── Has sudo access for:                          │
│         ├── systemctl restart/status/start/stop      │
│         ├── journalctl -u                            │
│         └── useradd                                  │
│                                                       │
│  my-api (system user, no shell)                      │
│    ├── Runs the actual application process           │
│    └── Created by admin, not by daemon               │
└─────────────────────────────────────────────────────┘
```

### Sudoers Configuration

The `init-server` command creates `/etc/sudoers.d/minideploy`:

```
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl status *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl start *
minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl stop *
minideploy ALL=(root) NOPASSWD: /usr/bin/journalctl -u *
minideploy ALL=(root) NOPASSWD: /usr/sbin/useradd *
minideploy ALL=(root) NOPASSWD: /usr/bin/pm2 *
```

This is deliberately narrow:
- No `systemctl daemon-reload` or `systemctl enable`
- No `systemctl kill` or `systemctl cancel` (dangerous)
- No root shell access
- No arbitrary file operations

The daemon does NOT need root for file operations because `/opt/<app>/` and `/var/lib/minideploy/` are owned by the `minideploy` user.

### Why Not Run as Root?

**Root**: Daemon as root would simplify `systemctl` calls but creates a larger attack surface. If the daemon is compromised, the attacker has full root access.

**minideploy user**: The daemon runs as an unprivileged system user with narrowly scoped sudo access. A compromise of the daemon only grants controlled process management capabilities.

---

## Network Security

### Localhost-Only Binding

The daemon listens on `127.0.0.1` (localhost), NOT `0.0.0.0`. This means:

- The API is **not accessible over the network**
- Only local processes (or SSH-forwarded connections) can reach it
- The daemon is invisible to port scans

### SSH Tunnel for Remote Access

To access the daemon from your development machine:

```bash
# Direct SSH tunnel
ssh -L 8443:127.0.0.1:8443 user@my-vps -N

# Or with autossh for persistence
autossh -M 0 -L 8443:127.0.0.1:8443 user@my-vps -N -o "ServerAliveInterval 30" -o "ServerAliveCountMax 3"
```

Then point your client at `127.0.0.1:8443`:

```bash
minideploy status --host 127.0.0.1 --port 8443
```

When `server.host` in `.deploy.yml` is an SSH alias that doesn't resolve via DNS, the client automatically starts and manages the SSH tunnel.

### API Key Storage

Best practices:

1. **Never commit** the API key to git
2. Use `.env` file (gitignored) or environment variables
3. Use a secrets manager (e.g., `pass`, 1Password CLI, Vault) in CI/CD

Example `.gitignore`:
```
.env
```

Example CI/CD (GitHub Actions):
```yaml
- name: Deploy
  env:
    MINIDEPLOY_API_KEY: ${{ secrets.MINIDEPLOY_API_KEY }}
  run: minideploy deploy
```

---

## Threat Model

| Threat | Mitigation |
|---|---|
| **API key leaked** | Rotate the key, bcrypt prevents offline cracking of the stored hash |
| **SSH key compromised** | Attacker can rsync files and reach the daemon via SSH tunnel, but still needs the API key |
| **Daemon process exploited** | Runs as unprivileged user with narrow sudo scope; cannot modify system units, enable services, or access root files |
| **state.json stolen** | API keys are bcrypt-hashed; attacker cannot extract the raw key |
| **Man-in-the-middle** | Daemon listens on localhost only; SSH tunnel is encrypted; no network exposure |

---

## Future Security Improvements

- **TLS/mTLS** for daemon API (certificate-based auth instead of bearer tokens)
- **Rate limiting** on auth endpoints
- **Audit logging** (log all API requests with timestamps and client IPs)
- **Key rotation command** (`minideploy rotate-key`)
- **Per-app API keys** (instead of a single shared key)
