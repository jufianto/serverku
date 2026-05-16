# Serverku + Caddyku Example Flow

This document outlines the exact user flow for how `serverku` and `caddyku` will work together to provision a server, deploy an app, and automatically route traffic to it with HTTPS.

## 1. Project Initialization

The user wants to deploy a new web application called "blog". They run the interactive initialization command on their local machine.

```bash
$ serverku init blog

Creating new project 'blog'...
? Which cloud provider? [gcp/digitalocean]: digitalocean
? Which region? [nyc1, sgp1, etc]: sgp1
? VM Size? [s-1vcpu-1gb]: s-1vcpu-1gb
? Enable persistent storage? [y/N]: y
? Storage size (GB) [10]: 10
? Path to your docker-compose file? [./docker-compose.yml]: ./docker-compose.yml
? Do you want to configure domains for automatic HTTPS? [y/N]: y

Domain Configuration:
? Main domain (e.g. blog.myapp.com): blog.jufi.dev
? Docker compose service name to route traffic to (e.g. web, backend): web
? Upstream port (e.g. web:8080): web:3000

Project 'blog' created successfully at ~/.serverku/projects/blog.yaml!
```

This generates the following `~/.serverku/projects/blog.yaml`:

```yaml
name: blog
provider: digitalocean
region: sgp1
vm:
  size: s-1vcpu-1gb
storage:
  enabled: true
  size_gb: 10
  mount_path: /data
compose_file: ./docker-compose.yml
router:
  enabled: true
  domains:
    - domain: blog.jufi.dev
      service: web
      upstream: web:3000
```

## 2. Deploying the Application

The user runs the `up` command from their local machine where their `docker-compose.yml` is located.

```bash
$ serverku up blog

Starting project "blog"...
  Provider: digitalocean
  Region:   sgp1
  VM:       s-1vcpu-1gb
  Storage:  10GB at /data
  Router:   blog.jufi.dev -> web:3000

[orchestrator] creating persistent disk "serverku-blog-data"...
[orchestrator] creating VM "serverku-blog"...
[orchestrator] attaching disk...
[orchestrator] VM ready. External IP: 159.223.45.67

[provisioner] connecting to 159.223.45.67 as serverku...
[provisioner] installing Docker...
[provisioner] mounting disk /data...
[provisioner] downloading caddyku to VM...
[provisioner] bootstrapping caddy-proxy network...
[provisioner] writing docker-compose.yml...
[provisioner] configuring caddyku domains...
[provisioner] starting containers...

Project "blog" is running!
  External IP: 159.223.45.67
  URL:         https://blog.jufi.dev

SSH: serverku ssh blog
```

## 3. What happens "under the hood" on the VM

During the `[provisioner]` phase, `serverku` establishes an SSH connection to the new VM and executes the following sequence of commands automatically:

1. **Install Docker:** `curl -fsSL https://get.docker.com | sh`
2. **Mount Storage:** Formats and mounts the block storage to `/data`.
3. **Install Caddyku:**
   ```bash
   curl -sSL https://github.com/jufianto/caddyku/releases/latest/download/caddyku_linux_amd64.tar.gz | tar -xz && sudo mv caddyku /usr/local/bin/
   ```
4. **Bootstrap Proxy:**
   ```bash
   caddyku init
   cd ~/projects/caddy-proxy && docker compose up -d
   ```
   *(Caddy is now running on ports 80/443 on the VM)*
5. **Transfer Compose File:** The user's local `./docker-compose.yml` is copied to `/data/docker-compose.yml` on the VM.
6. **Wire up the Application:**
   Because the user configured `router.enabled = true` in their `blog.yaml`, `serverku` runs:
   ```bash
   cd /data
   caddyku init-app --service web --domain blog.jufi.dev --upstream web:3000
   ```
   *This automatically modifies `/data/docker-compose.yml` to attach the `web` service to the `caddy-net` network, creates the `caddyku.yaml`, and reloads the global Caddy instance.*
7. **Start the Application:**
   ```bash
   cd /data && docker compose up -d
   ```

## 4. Result

The user's application is now live on `https://blog.jufi.dev`. Caddy automatically provisions the Let's Encrypt SSL certificate. 

If the user runs `serverku down blog`, the VM is destroyed, but the `/data` disk (containing their modified `docker-compose.yml` and `caddyku.yaml`) is preserved.

If they run `serverku up blog` again later, the system will re-attach the disk, reinstall `caddyku`, start the proxy, and because the files are already configured on the disk, the site will come back online immediately.
