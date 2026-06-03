# Deployment Walkthrough

This guide walks through three real-world deployments end-to-end. Each example shows you how to set up the server-side service first, then configure and run minideploy.

## Common Setup

For all examples, assume:

- Your VPS is running Ubuntu/Debian with systemd
- You've already run `minideploy init-server --host my-vps --ssh-user root`
- You have an API key from that output
- You've tested the daemon: `curl http://127.0.0.1:8443/api/v1/health`

---

## Example 1: Deploying a Go HTTP Server

### The App

A simple Go HTTP server that reads `PORT` from the environment:

```go
// main.go
package main

import (
    "fmt"
    "net/http"
    "os"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello from port %s\n", port)
    })

    http.ListenAndServe(":"+port, nil)
}
```

### Step 1: Create a systemd service for the app

On the VPS, create `/etc/systemd/system/my-api@.service`:

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

The `@` in the filename (`my-api@.service`) makes it a template unit. `%i` is replaced with the instance ID (e.g., `3000`). The `ExecStart` points to `/opt/my-api/current/app` — a symlink that minideploy manages.

Create the app user:

```bash
sudo useradd --system --no-create-home --shell /sbin/nologin my-api
```

Enable the instances:

```bash
sudo systemctl enable my-api@3000
sudo systemctl enable my-api@3001
```

### Step 2: Create `.deploy.yml` in your Go project

```yaml
app_name: my-api
service_type: systemd
service_name: my-api@%i
instances:
  - id: "3000"
    port: 3000
    env:
      PORT: 3000
  - id: "3001"
    port: 3001
    env:
      PORT: 3001
deploy_path: /opt/my-api
build:
  - go build -o app .
artifacts:
  - app
server:
  host: my-vps
  api_port: 8443
  ssh_user: root
  api_key: <your-api-key>
```

### Step 3: Deploy

```bash
$ minideploy deploy

[deploy] starting deployment for my-api
[build] (1/1) go build -o app .
[build] all 1 steps completed
[rsync] rsync -avz --delete app root@my-vps:/opt/my-api/upload/
sending incremental file list
app
              sent 8.2M bytes  received 48 bytes  1.6M bytes/sec

[deploy] release 20260603-150000 deployed successfully
[deploy] instances restarted: [my-api@3000 my-api@3001]
```

### Step 4: Verify

```bash
$ curl http://my-vps:3000
Hello from port 3000

$ curl http://my-vps:3001
Hello from port 3001

$ minideploy ps
my-api              running   20260603-150000
  └─ my-api@3000  ●  (port 3000)
  └─ my-api@3001  ●  (port 3001)
```

### Step 5: Deploy a new version

Make a change to your code, rebuild, and run `minideploy deploy` again. The daemon creates a new release and atomically swaps the symlink before restarting.

---

## Example 2: Deploying an Express API

### The App

A Node.js Express API:

```javascript
// server.js
const express = require('express');
const app = express();
const port = process.env.PORT || 3000;

app.get('/', (req, res) => {
  res.json({ message: `Hello from port ${port}` });
});

app.listen(port, () => {
  console.log(`Listening on port ${port}`);
});
```

```json
// package.json
{
  "name": "express-api",
  "version": "1.0.0",
  "scripts": {
    "start": "node server.js"
  },
  "dependencies": {
    "express": "^4.18.0"
  }
}
```

### Step 1: systemd service

`/etc/systemd/system/express-api@.service`:

```ini
[Unit]
Description=Express API (instance %i)
After=network.target

[Service]
Type=simple
User=express-api
WorkingDirectory=/opt/express-api/current
ExecStart=/usr/bin/node /opt/express-api/current/server.js
Restart=always
RestartSec=3
EnvironmentFile=/opt/express-api/env-%i.conf

[Install]
WantedBy=multi-user.target
```

### Step 2: `.deploy.yml`

```yaml
app_name: express-api
service_type: systemd
service_name: express-api@%i
instances:
  - id: "4000"
    port: 4000
    env:
      PORT: 4000
  - id: "4001"
    port: 4001
    env:
      PORT: 4001
deploy_path: /opt/express-api
build:
  - npm ci
artifacts:
  - server.js
  - package.json
  - node_modules/
server:
  host: my-vps
  api_port: 8443
  ssh_user: root
  api_key: <your-api-key>
env:
  NODE_ENV: production
```

### Step 3: Deploy

```bash
$ minideploy deploy

[deploy] starting deployment for express-api
[build] (1/1) npm ci
added 62 packages in 1.2s
[rsync] rsync -avz --delete server.js package.json node_modules/ root@my-vps:/opt/express-api/upload/
...
[deploy] release 20260603-151000 deployed successfully
[deploy] instances restarted: [express-api@4000 express-api@4001]
```

---

## Example 3: Deploying a .NET Core Web API

### The App

A minimal ASP.NET Core Web API:

```bash
dotnet new webapi -n WebApi
```

`Program.cs`:

```csharp
var builder = WebApplication.CreateBuilder(args);
builder.WebHost.UseUrls($"http://0.0.0.0:{builder.Configuration["PORT"] ?? "5000"}");
var app = builder.Build();
app.MapGet("/", () => "Hello from .NET");
app.Run();
```

### Step 1: Publish script

Add a build step that produces a framework-dependent deployment:

```bash
# publish.sh
dotnet publish -c Release -o publish
```

### Step 2: systemd service

`/etc/systemd/system/webapi@.service`:

```ini
[Unit]
Description=WebApi .NET (instance %i)
After=network.target

[Service]
Type=simple
User=webapi
WorkingDirectory=/opt/webapi/current/publish
ExecStart=/usr/bin/dotnet /opt/webapi/current/publish/WebApi.dll
Restart=always
RestartSec=3
EnvironmentFile=/opt/webapi/env-%i.conf

[Install]
WantedBy=multi-user.target
```

> **Note**: This example uses framework-dependent deployment. The .NET runtime must be installed on the server. For self-contained deployment, use `dotnet publish -c Release -r linux-x64 --self-contained -o publish`.

### Step 3: `.deploy.yml`

```yaml
app_name: webapi
service_type: systemd
service_name: webapi@%i
instances:
  - id: "5000"
    port: 5000
    env:
      PORT: 5000
deploy_path: /opt/webapi
build:
  - dotnet publish -c Release -o publish
artifacts:
  - publish/
server:
  host: my-vps
  api_port: 8443
  ssh_user: root
  api_key: <your-api-key>
```

### Step 4: Deploy

```bash
$ minideploy deploy

[deploy] starting deployment for webapi
[build] (1/1) dotnet publish -c Release -o publish
  Determining projects to restore...
  All projects are up-to-date for restore.
  WebApi -> /home/user/project/publish/WebApi.dll
  Build succeeded.
[rsync] rsync -avz --delete publish/ root@my-vps:/opt/webapi/upload/
...
[deploy] release 20260603-152000 deployed successfully
[deploy] instances restarted: [webapi@5000]
```

---

## What Happens During a Deploy

Here's the concrete output on the server during the Go example deploy:

```bash
# Before deploy
$ ls -la /opt/my-api/
drwxr-xr-x  upload/
drwxr-xr-x  releases/
             └── 20260602-120000/
                     └── app
lrwxrwxrwx  current -> releases/20260602-120000/

# During deploy (rsync writes to upload/)
$ ls -la /opt/my-api/upload/
-rwxr-xr-x  app   # new binary

# After deploy daemon finishes
$ ls -la /opt/my-api/
drwxr-xr-x  upload/
drwxr-xr-x  releases/
             ├── 20260602-120000/
             │       └── app       # old binary
             └── 20260603-150000/
                     └── app       # new binary
lrwxrwxrwx  current -> releases/20260603-150000/  # atomically swapped
```
