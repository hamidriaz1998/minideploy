# Installation

## Prerequisites

- **Go 1.21+** (for building from source)
- **SSH access** to your VPS with `root` or a sudo-capable user
- **rsync** installed on both the development machine and the server
- **systemd** or **pm2** on the server (depending on your setup)

## Building from Source

```bash
git clone <your-repo> && cd minideploy
make build
```

The binary will be at `build/minideploy`.

Or build directly:

```bash
go build -o minideploy .
```

Cross-compile for a linux server:

```bash
GOOS=linux GOARCH=amd64 go build -o minideploy-linux .
```

## Installing the Daemon on a VPS

### Quick Method: `init-server`

The `init-server` command automates the full VPS setup:

```bash
minideploy init-server --host my-vps --ssh-user root
```

This will:

1. Cross-compile the daemon for `linux/amd64`
2. SCP the binary to `/usr/local/bin/minideploy`
3. Create a `minideploy` system user
4. Create the directory structure (`/opt/<app>/upload`, `releases`, `/var/lib/minideploy`)
5. Set up a systemd service file at `/etc/systemd/system/minideploy.service`
6. Configure sudoers for the `minideploy` user
7. Generate an API key
8. Start the daemon

Output:

```
═══════════════════════════════════════════
  Daemon installed!

  Host:      my-vps
  API Port:  8443

  API Key:
  a1b2c3d4e5f6... (64 hex chars)

  Add to .deploy.yml:
  server:
    host: my-vps
    api_port: 8443
    ssh_user: root
    api_key: a1b2c3d4e5f6...
═══════════════════════════════════════════
```

Save the API key in your `.deploy.yml`, `.env` file, or as an environment variable.

### Adding More Apps

If the daemon is already running and you want to register another app:

```bash
minideploy init-server --host my-vps --app-name another-app --deploy-path /opt/another-app
```

This creates the directory structure and reuses the existing daemon.

### Manual Setup

If you prefer to set things up by hand:

1. **Create the system user**:

   ```bash
   sudo useradd --system --no-create-home --shell /sbin/nologin minideploy
   ```

2. **Create directories**:

   ```bash
   sudo mkdir -p /opt/<app>/upload /opt/<app>/releases /var/lib/minideploy
   sudo chown -R minideploy:minideploy /opt/<app> /var/lib/minideploy
   ```

3. **Install the binary**:

   ```bash
   sudo cp minideploy-linux /usr/local/bin/minideploy
   sudo chmod 755 /usr/local/bin/minideploy
   ```

4. **Create the daemon systemd service** (`/etc/systemd/system/minideploy.service`):

   ```ini
   [Unit]
   Description=minideploy Daemon
   After=network.target

   [Service]
   Type=simple
   User=minideploy
   Group=minideploy
   ExecStart=/usr/local/bin/minideploy daemon --state-dir /var/lib/minideploy
   Restart=always
   RestartSec=5
   StateDirectory=minideploy

   [Install]
   WantedBy=multi-user.target
   ```

5. **Configure sudoers** (`/etc/sudoers.d/minideploy`):

   ```
   minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart *
   minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl status *
   minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl start *
   minideploy ALL=(root) NOPASSWD: /usr/bin/systemctl stop *
   minideploy ALL=(root) NOPASSWD: /usr/bin/journalctl -u *
   minideploy ALL=(root) NOPASSWD: /usr/sbin/useradd *
   minideploy ALL=(root) NOPASSWD: /usr/bin/pm2 *
   ```

6. **Start the daemon**:

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now minideploy
   ```

7. **Generate an API key**:

   The daemon generates one automatically on first start. Check the logs:

   ```bash
   sudo journalctl -u minideploy -n 20 --no-pager
   ```

## Upgrading the Daemon

```bash
# Build the new version
go build -o minideploy-linux .

# Copy it to the server
scp minideploy-linux root@my-vps:/usr/local/bin/minideploy

# Restart the daemon
ssh root@my-vps systemctl restart minideploy
```

## Verifying the Installation

```bash
# Check if daemon is running
ssh root@my-vps systemctl status minideploy

# Test the health endpoint (no auth required)
curl http://127.0.0.1:8443/api/v1/health
# → {"success":true,"data":{"status":"ok"}}

# Test with your API key
curl -H "Authorization: Bearer <your-key>" http://127.0.0.1:8443/api/v1/status
```
