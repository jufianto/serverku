## Context

Deploying Docker containers directly exposes them to the internet on their specified ports, usually without HTTPS. Users expect applications to be securely served over HTTPS (port 443) using their own domain names. We can solve this by integrating `caddyku`, a CLI wrapper around Caddy designed to manage a shared reverse proxy on a single VPS. By integrating `caddyku` into `serverku`, we can automate the installation and configuration of a reverse proxy during VM provisioning.

## Goals / Non-Goals

**Goals:**
- Add a `Router` block to the `ProjectConfig` to define domain routing rules.
- During provisioning (`serverku up`), automatically download and install `caddyku` on the remote VM.
- Run `caddyku init` on the VM to bootstrap the shared Caddy proxy container and network (`caddy-net`).
- Run `caddyku init-app` for each configured domain to modify the project's `docker-compose.yml` to join `caddy-net` and register the routing rules in the Caddyfile.

**Non-Goals:**
- We are not rewriting Caddy or `caddyku` logic in Go within `serverku`. We will execute the `caddyku` binary on the VM.
- We will not automatically configure DNS records at the DNS provider. The user is still responsible for pointing their domain's A-record to the VM's external IP address.

## Decisions

**1. Router Configuration Structure:**
The `ProjectConfig` will have a `Router` block:
```yaml
router:
  enabled: true
  domains:
    - domain: example.com
      service: web
      upstream: web:8080
```
- *Rationale*: This maps directly to the arguments needed by `caddyku init-app`.

**2. Provisioner Execution Flow:**
The `SSHProvisioner` will execute the following steps if `Router.Enabled` is true:
1. Download `caddyku` release tarball via `curl`.
2. Extract and move to `/usr/local/bin/caddyku`.
3. Run `caddyku init`.
4. Start the `caddy-proxy` stack: `cd ~/projects/caddy-proxy && docker compose up -d`.
5. For each domain in the config, run `caddyku init-app --service <service> --domain <domain> --upstream <upstream>` within the project's deployment directory.

**3. Idempotency:**
The provisioning commands must be safe to run multiple times (e.g., during subsequent `serverku up` runs). `caddyku init` and `caddyku init-app` handle existing setups gracefully.

## Risks / Trade-offs

- **[Risk] `caddyku` release URL changes or goes down** → *Mitigation*: Hardcode a known working version URL or point to the "latest" release endpoint on GitHub. If GitHub is unreachable, provisioning will fail, but this is a standard risk for curl-based installs.
- **[Risk] Port conflicts (80/443)** → *Mitigation*: The `caddy-proxy` binds to ports 80 and 443. The user's application must not bind these ports directly in their `docker-compose.yml`. `caddyku` helps avoid this by routing traffic internally over `caddy-net`.
