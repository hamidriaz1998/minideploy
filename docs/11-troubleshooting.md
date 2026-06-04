# Troubleshooting

## Common Issues

### Daemon Won't Start

```bash
$ systemctl status minideploy
● minideploy.service - minideploy Daemon
     Loaded: loaded (/etc/systemd/system/minideploy.service; enabled)
     Active: failed (Result: exit-code)
```

**Check logs**:

```bash
journalctl -u minideploy -n 50 --no-pager
```

**Common causes**:

| Symptom | Likely cause | Fix |
|---|---|---|
| `permission denied` | Binary not executable | `chmod 755 /usr/local/bin/minideploy` |
| `state.json: permission denied` | State dir ownership | `chown -R minideploy:minideploy /var/lib/minideploy` |
| `unknown service` or missing command | Binary not properly installed | Re-run `init-server` or verify `minideploy daemon --help` works |
| `bind: address already in use` | Port conflict | Change port with `--port` flag |

---

### SSH Tunnel Fails

```bash
$ minideploy deploy
error: ssh tunnel: exit status 255
```

**Debug**:

```bash
# Test the SSH connection directly
ssh -v root@my-vps

# Test the tunnel manually
ssh -L 8443:127.0.0.1:8443 root@my-vps -N -v
```

**Common causes**:

| Issue | Fix |
|---|---|
| SSH key not added | `ssh-add ~/.ssh/your-key` |
| Wrong user in config | Check `server.ssh_user` in `.deploy.yml` |
| Host not in `~/.ssh/config` | Add the host entry or use an IP address |
| SSH server not running on VPS | `systemctl status sshd` |

---

### Rsync Fails

```bash
$ minideploy upload
[rsync] rsync -avz --delete ... ssh: connect to host my-vps port 22: Connection refused
rsync: connection unexpectedly closed (0 bytes received so far) [sender]
rsync error: error in rsync protocol data stream
```

**Diagnose**:

```bash
# Can you SSH in?
ssh root@my-vps

# Is rsync installed on the server?
ssh root@my-vps which rsync

# Is the upload directory writable?
ssh root@my-vps ls -la /opt/my-app/upload/
```

**Common causes**:

| Issue | Fix |
|---|---|
| SSH user wrong | Check `server.ssh_user` in `.deploy.yml` |
| rsync not installed on server | `ssh root@my-vps apt install rsync` |
| Deploy path doesn't exist | Run `init-app` or `mkdir -p /opt/<app>/upload` |
| Permission denied on write | `chown -R minideploy:minideploy /opt/<app>` |

---

### Deploy Succeeds but Services Don't Restart

If the API response shows `"some instances failed"`:

```json
{
  "success": false,
  "data": { "release": "20260603-143022", "instances": [], "app_name": "my-api" },
  "error": "some instances failed: my-api@3000"
}
```

**Check**:

```bash
# Does the systemd unit exist?
ssh root@my-vps systemctl status my-api@3000

# Can the daemon run systemctl?
ssh root@my-vps sudo -u minideploy sudo systemctl status my-api@3000

# Is the sudoers file correct?
ssh root@my-vps cat /etc/sudoers.d/minideploy
```

**Common causes**:

| Issue | Fix |
|---|---|
| systemd unit doesn't exist | Create `my-api@.service` in `/etc/systemd/system/` |
| Sudoers file missing or wrong | Re-run `init-server` or check `/etc/sudoers.d/minideploy` |
| Binary not executable | `chmod +x /opt/my-api/current/app` |
| Wrong `%i` in service_name | Ensure `service_name` in `.deploy.yml` matches your unit file name |

---

### API Key Issues

```bash
$ minideploy status
error: request failed: 401 Unauthorized
```

**Check**:

```bash
# Is the key set?
echo $MINIDEPLOY_API_KEY
grep api_key .deploy.yml

# Does the daemon have a key stored?
ssh root@my-vps cat /var/lib/minideploy/state.json | grep key_hash
```

**Common causes**:

| Issue | Fix |
|---|---|
| Key not set | Set `server.api_key`, `MINIDEPLOY_API_KEY` env, or `.env` |
| Key wrong | The key in `.deploy.yml` may be from a different daemon instance |
| State file empty or corrupt | Restart daemon to auto-generate a new key |

If you've lost the API key and the daemon is running:

```bash
# Check first-start logs
sudo journalctl -u minideploy | grep "Generated one-time key"

# Or manually check state
sudo cat /var/lib/minideploy/state.json | python3 -c "import sys,json; print(json.load(sys.stdin)['api_keys'][0]['key_hash'])"
# (this shows the hash, not the raw key — you'll need to generate a new one)
```

---

### Deploy Hangs or Times Out

```bash
$ minideploy deploy
[build] (1/1) go build -o app .
# ... nothing for 60 seconds
```

**Possible causes**:

1. **Build is legitimately slow** — some builds (e.g., `npm install`, `dotnet restore`) take minutes
2. **Network issue** — rsync hangs on slow connections; add `-v` to the rsync command for debugging
3. **Daemon not responding** — check if the daemon is running: `ssh root@my-vps systemctl status minideploy`
4. **SSH tunnel not cleaned up** — stale SSH process from a previous run; `pkill -f "ssh.*8443"`

---

### Permission Errors During Hard Destroy

If `minideploy destroy --confirm` fails with permission errors:

```bash
# The daemon's deploy_path must be owned by minideploy
ssh root@my-vps chown -R minideploy:minideploy /opt/<app>

# If that doesn't work, run as root manually
ssh root@my-vps rm -rf /opt/<app>
```

---

## Debugging Commands

```bash
# Check daemon health (no auth needed on localhost)
curl http://127.0.0.1:8443/api/v1/health

# Check daemon status with auth
curl -H "Authorization: Bearer <key>" http://127.0.0.1:8443/api/v1/status

# List all apps
curl -H "Authorization: Bearer <key>" http://127.0.0.1:8443/api/v1/apps | jq .

# View daemon logs
sudo journalctl -u minideploy -f

# Check what's in the upload directory
ssh root@my-vps ls -la /opt/my-app/upload/

# Check current symlink
ssh root@my-vps readlink -f /opt/my-app/current
```

## Getting Help

If you encounter a bug or need help, open an issue at the project repository with:

- The output of `minideploy --version`
- The command you ran
- The full error output
- Relevant daemon logs: `journalctl -u minideploy -n 50 --no-pager`
