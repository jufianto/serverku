# serverku

`serverku` is an on-demand cloud VM orchestrator for Docker Compose projects.

It is designed for side projects, previews, staging environments, demos, and internal tools that do not need to run 24/7. Keep your application data on persistent block storage, turn compute on only when you need it, and tear the VM down when you are done.

## Why serverku?

Cloud VMs are billed while they are running, even when nobody is using them. For many projects, the expensive part is not storage, it is idle compute.

`serverku` treats VMs as disposable runtime. Your project config and optional persistent disk survive between sessions. When you run `serverku up`, it creates a VM, attaches storage, provisions Docker, syncs your project, starts your Compose stack, and gives you a reachable server. When you run `serverku down`, it stops the workload and destroys the VM while keeping your data.

## Features

- **On-demand VMs**: Create and destroy cloud VMs with simple CLI commands.
- **Persistent storage**: Keep project data on block storage while compute is off.
- **Docker Compose deployment**: Deploy existing Compose stacks without adopting a platform-specific format.
- **Project sync**: Sync full project directories with `rsync`, including `.env`, configs, and build context.
- **Automatic HTTPS routing**: Integrate with [`caddyku`](https://github.com/jufianto/caddyku) to route domains to Compose services with Caddy and Let's Encrypt.
- **DNS automation**: Optionally create/update A records for your domains on `up` (DigitalOcean).
- **Interactive setup**: Create project configs through a guided `serverku init` wizard.
- **GCP and DigitalOcean providers**: Provision Compute Engine instances or DigitalOcean Droplets.
- **SSH utilities**: Open shells, stream Compose logs, and create secure port tunnels through managed SSH keys.
- **Notifications**: Send lifecycle updates to Slack and Telegram.
- **Cost estimates**: Show rough local VM/storage cost estimates in CLI output.

## Status

`serverku` is early-stage software. The core workflow exists, but real cloud provisioning should be tested carefully in throwaway projects before using it for important workloads.

The current pricing feature is an offline rough estimate, not provider billing data. Real provider-backed pricing is planned.

## Install

### From source

```bash
git clone https://github.com/jufianto/serverku.git
cd serverku
go build -o serverku ./cmd/serverku
```

Then run:

```bash
./serverku --help
```

### Go install

```bash
go install github.com/jufianto/serverku/cmd/serverku@latest
```

## Requirements

- Go 1.21+ for building from source.
- `ssh` installed locally.
- `rsync` installed locally when using `sync_dir`.
- A cloud account for real deployments.
- GCP Application Default Credentials for GCP deployments.
- `DIGITALOCEAN_TOKEN` for DigitalOcean deployments.
- A Docker Compose project to deploy.
- Optional: a domain pointing at the VM IP when using Caddyku routing.

## Quick Start

Create a project config:

```bash
serverku init myapp
```

Start the project:

```bash
serverku up myapp
```

Check state and IP:

```bash
serverku status myapp
```

SSH into the VM:

```bash
serverku ssh myapp
```

Stream application logs:

```bash
serverku logs myapp
```

Open a secure tunnel to an internal service:

```bash
serverku tunnel myapp 5432:5432
```

Stop compute while keeping storage:

```bash
serverku down myapp
```

Delete everything permanently:

```bash
serverku destroy myapp
```

## Typical Workflow

### 1. Prepare your Docker Compose project

Your application can be any Docker Compose project:

```text
myapp/
  docker-compose.yml
  .env
  src/
  config/
```

Example Compose service:

```yaml
services:
  web:
    build: .
    env_file: .env
    expose:
      - "3000"
```

### 2. Initialize serverku config

```bash
serverku init myapp
```

The interactive wizard creates a config under:

```text
~/.serverku/projects/myapp.yaml
```

It also ensures the managed SSH keypair exists under:

```text
~/.serverku/keys/serverku_rsa
~/.serverku/keys/serverku_rsa.pub
```

### 3. Configure sync and routing

Point `sync_dir` at your local project directory. Configure `router` if you want HTTPS through Caddyku.

```yaml
name: myapp
provider: digitalocean
region: sgp1

vm:
  size: s-1vcpu-1gb
  image: ubuntu-22-04-x64
  spot: false

storage:
  enabled: true
  size_gb: 10
  mount_path: /data

compose_file: ./myapp/docker-compose.yml
sync_dir: ./myapp

router:
  enabled: true
  domains:
    - domain: myapp.example.com
      service: web
      upstream: web:3000
```

### 4. Bring the server online

```bash
serverku up myapp
```

What happens:

- Creates a VM.
- Creates or attaches persistent storage.
- Waits until SSH is ready.
- Installs Docker.
- Mounts storage at the configured mount path.
- Syncs `sync_dir` to the VM using `rsync`.
- Writes the Compose file.
- Installs and initializes Caddyku if routing is enabled.
- Runs `docker compose up -d`.
- Prints the VM IP and rough cost estimate.

### 5. Operate the app

```bash
serverku status myapp
serverku list
serverku logs myapp
serverku ssh myapp
serverku tunnel myapp 5432:5432
```

### 6. Turn compute off

```bash
serverku down myapp
```

The VM is destroyed. Persistent storage remains available for the next `serverku up`.

### 7. Delete the project

```bash
serverku destroy myapp
```

This removes cloud resources and local project state/config. Treat it as irreversible.

## Commands

| Command | Description |
| --- | --- |
| `serverku init <project>` | Create a project config and SSH keys. |
| `serverku up <project>` | Create VM, attach storage, provision, sync, and deploy. |
| `serverku down <project>` | Destroy VM while preserving persistent storage. |
| `serverku destroy <project>` | Delete VM, storage, state, and config. |
| `serverku status <project>` | Show project status and reconcile with provider. |
| `serverku list` | List all projects with status and estimated costs. |
| `serverku ssh <project>` | Open an interactive SSH shell. |
| `serverku logs <project>` | Stream remote `docker compose logs -f`. |
| `serverku tunnel <project> <local>:<remote>` | Open an SSH port-forwarding tunnel. |

Global flags:

```bash
--config-dir string   config directory (default: ~/.serverku/)
-v, --verbose         enable verbose output
--version             print version
```

## Configuration Reference

Example with most supported fields:

```yaml
name: myapp
provider: digitalocean # gcp or digitalocean
project_id: ""         # required for gcp
region: sgp1
zone: ""               # required for gcp

vm:
  size: s-1vcpu-1gb
  image: ubuntu-22-04-x64
  spot: false

storage:
  enabled: true
  size_gb: 10
  mount_path: /data

compose_file: ./docker-compose.yml
sync_dir: ./myapp

startup_commands:
  - "docker compose ps"

router:
  enabled: true
  domains:
    - domain: myapp.example.com
      service: web
      upstream: web:3000

dns:
  enabled: true   # auto-manage A records for router.domains on `up`
  ttl: 3600       # record TTL in seconds (default 3600)

notifications:
  slack:
    webhook_url: "https://hooks.slack.com/services/..."
  telegram:
    bot_token: "123456:ABC"
    chat_id: "123456789"
```

## Providers

| Provider | Status | Auth |
| --- | --- | --- |
| GCP | Implemented | Application Default Credentials |
| DigitalOcean | Implemented | `DIGITALOCEAN_TOKEN` |

DigitalOcean does not support `spot: true` in the same way GCP supports preemptible instances. The DigitalOcean provider returns an error for Spot requests.

## Caddyku Routing

`serverku` can use `caddyku` on the VM to provide automatic HTTPS routing.

When `router.enabled` is true, provisioning will:

- Download and install the `caddyku` binary.
- Run `caddyku init`.
- Start the global Caddy proxy stack.
- Run `caddyku init-app` for each configured domain.
- Start the Compose application.

Point your DNS records at the VM external IP, or enable DNS automation (see below) to have `serverku` do it for you.

## DNS Automation

When `dns.enabled` is true, `serverku up` creates or updates an A record for each
`router.domains` entry, pointing it at the VM's external IP. This happens before
provisioning so Caddyku/Let's Encrypt can resolve the domain when issuing
certificates.

```yaml
router:
  enabled: true
  domains:
    - domain: myapp.example.com
      service: web
      upstream: web:3000
dns:
  enabled: true
  ttl: 3600
```

Notes:

- **DigitalOcean only** for now. The domain's nameservers must already be
  delegated to DigitalOcean (the apex zone must exist in your DO account);
  `serverku` matches each FQDN to the longest managed zone and writes the
  record. GCP DNS automation is planned — enabling `dns` on a GCP project fails
  fast with a clear error rather than silently skipping.
- Record changes are idempotent: a record already pointing at the IP is left
  unchanged.
- A DNS write failure aborts `up` and tears down the VM (persistent storage is
  preserved).
- Freshly created records may take time to propagate; Caddy retries certificate
  issuance, but a low `ttl` helps during initial setup.

## Cost Estimates

`serverku` currently uses local hardcoded pricing tables to show rough estimates.

It does not call cloud billing APIs yet. Estimates do not include taxes, bandwidth, snapshots, discounts, promotions, reserved pricing, or region-specific differences unless encoded in the local tables.

Real provider-backed pricing is planned.

## Local Development

Build:

```bash
go build -o serverku ./cmd/serverku
```

Run tests:

```bash
go test ./...
```

Run a focused package:

```bash
go test ./internal/orchestrator
go test ./internal/pricing
go test ./internal/provider/digitalocean
```

Use a temporary config directory while testing locally:

```bash
tmpdir="$(mktemp -d)"
./serverku --config-dir "$tmpdir" init demo --non-interactive --provider digitalocean --region sgp1 --size s-1vcpu-1gb --no-storage
./serverku --config-dir "$tmpdir" list
```

## Testing Without Cloud Access

You can verify a lot without creating real cloud resources:

```bash
go build -o serverku ./cmd/serverku
go test ./...
./serverku help
```

Local tests cover config, orchestration logic, pricing estimates, notification composition, and some provider helper behavior.

See `INTERNAL_README.md` for a deeper feature-by-feature verification matrix and offline testing plan.

## Safety Notes

- `serverku up` can create billable cloud resources.
- `serverku down` destroys the VM but keeps persistent storage.
- `serverku destroy` deletes project resources and should be treated as irreversible.
- Use small instances and throwaway projects for testing.
- Always confirm resources are cleaned up in your cloud provider console after experiments.

## Roadmap

- Real provider-backed pricing.
- DNS automation for GCP (DigitalOcean is implemented).
- Snapshot and backup management.
- Local pre/post hooks.
- More offline command/script tests.
- Additional cloud providers later.

## License

MIT
