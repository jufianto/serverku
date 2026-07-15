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
- **Local hooks**: Run local commands around the lifecycle (e.g. build assets before `up`, clean up after `down`).
- **Automatic HTTPS routing**: Integrate with [`caddyku`](https://github.com/jufianto/caddyku) to route domains to Compose services with Caddy and Let's Encrypt.
- **DNS automation**: Optionally create/update A records for your domains on `up` (GCP and DigitalOcean).
- **Snapshots**: Back up a project's persistent disk with `serverku backup`.
- **Interactive setup**: Create project configs through a guided `serverku init` wizard.
- **GCP and DigitalOcean providers**: Provision Compute Engine instances or DigitalOcean Droplets.
- **SSH utilities**: Open shells, stream Compose logs, and create secure port tunnels through managed SSH keys.
- **Notifications**: Send lifecycle updates to ntfy push, Slack, and Telegram, plus an optional on-VM heartbeat that reminds you a VM is still running (and what it has cost so far).
- **Real cost visibility**: Live per-VM prices from the provider APIs (DigitalOcean sizes, GCP Billing Catalog), accrued session cost in `status`/`list`, and DigitalOcean month-to-date account usage. Offline table estimates are used as fallback and always marked `est.`.

## Status

`serverku` is early-stage software. The core workflow exists, but real cloud provisioning should be tested carefully in throwaway projects before using it for important workloads.

Cost figures prefer live provider pricing APIs and clearly mark offline fallback estimates with `est.` — an estimate is never presented as a bill.

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

- Go 1.23+ for building from source.
- `ssh` installed locally.
- `rsync` installed locally when using `sync_dir`.
- A cloud account for real deployments.
- GCP Application Default Credentials for GCP deployments.
- `DIGITALOCEAN_TOKEN` for DigitalOcean deployments.
- A Docker Compose project to deploy.
- Optional: a domain pointing at the VM IP when using Caddyku routing.

## Tutorials

Prefer learning by doing? [`docs/`](docs/README.md) has blog-style
walkthroughs deploying real open-source apps, from beginner to advanced:

1. [Uptime Kuma (JS) on DigitalOcean in 10 minutes](docs/tutorial-uptime-kuma-digitalocean.md) — the minimal first deploy.
2. [WordPress (PHP) with HTTPS, DNS automation, and persistent data](docs/tutorial-wordpress-digitalocean.md) — the full production flow.
3. [Gitea (Go) on GCP spot instances with snapshots](docs/tutorial-gitea-gcp.md) — cheap interruptible compute done right.
4. [Notifications: never pay for a forgotten VM](docs/tutorial-notifications.md) — ntfy, the Telegram wizard, and the still-running heartbeat.

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

For scripting, skip the wizard with `--non-interactive` and pass everything as flags:

```bash
serverku init myapp --non-interactive \
  --provider digitalocean --region sgp1 --size s-1vcpu-1gb --no-storage
```

| Flag | Description |
| --- | --- |
| `-p, --provider` | Cloud provider (`gcp`, `digitalocean`). |
| `--project-id` | Cloud project ID (required for GCP). |
| `-r, --region` | Cloud region. |
| `-z, --zone` | Cloud zone (required for GCP). |
| `-s, --size` | VM machine type (default `e2-medium`). |
| `--spot` | Use SPOT/preemptible instances (GCP only; defaults to `true` for GCP and `false` for DigitalOcean, which rejects it). |
| `--no-storage` | Create a fully stateless project without a persistent disk. |
| `--storage-gb` | Persistent disk size in GB (default `20`). |
| `--non-interactive` | Skip the interactive wizard. |

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
- Prints the VM IP and hourly cost (live provider price when available).

### 5. Operate and iterate

```bash
serverku status myapp
serverku list
serverku logs myapp
serverku ssh myapp
serverku tunnel myapp 5432:5432
```

Changed your code? Push it to the running VM without recreating anything:

```bash
serverku deploy myapp
```

Same VM, same IP, no DNS churn — just re-sync + `docker compose up -d`,
so only changed services restart. (`up` is for creating the VM; `deploy`
is for iterating on it.)

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
| `serverku deploy <project>` | Push code changes to the running VM: re-sync, rewrite compose, `compose up -d`. Same IP, seconds not minutes. |
| `serverku down <project>` | Destroy VM while preserving persistent storage. Use `-f/--force` to skip the confirmation prompt. |
| `serverku destroy <project>` | Delete VM, storage, state, and config. Use `-f/--force` to skip the confirmation prompt. |
| `serverku status <project>` | Show project status and reconcile with provider. |
| `serverku list` | List all projects with status and estimated costs. |
| `serverku ssh <project>` | Open an interactive SSH shell. |
| `serverku logs <project>` | Stream remote `docker compose logs -f`. |
| `serverku tunnel <project> <local>:<remote>` | Open an SSH port-forwarding tunnel. |
| `serverku backup <project>` | Snapshot the project's persistent disk. |
| `serverku restore <project> <snapshot>` | Restore a disk from a snapshot and point the project at it. |
| `serverku ntfy <project>` | Show how to subscribe to push notifications; `--test` sends a test message. |
| `serverku notify setup <project>` | Interactive wizard: connect ntfy or Telegram and verify with a real test send. |
| `serverku notify test <project>` | Send a test notification to every configured channel. |

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

hooks:                       # commands run on your LOCAL machine
  pre_up:
    - npm ci
    - npm run build
  post_down:
    - ./scripts/cleanup-local.sh

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
  ntfy:
    topic: serverku-myapp-8d17e4e4   # generated by init; see `serverku ntfy myapp`
    heartbeat_hours: 6               # on-VM reminder every 6h while running
    # server: https://ntfy.example.com   # optional self-hosted server
  slack:
    webhook_url: "https://hooks.slack.com/services/..."
  telegram:
    bot_token: "123456:ABC"
    chat_id: "123456789"
    heartbeat_hours: 6   # on-VM reminder via Telegram (token stored on the VM)
```

## Providers

| Provider | Status | Auth |
| --- | --- | --- |
| GCP | Implemented | Application Default Credentials |
| DigitalOcean | Implemented | `DIGITALOCEAN_TOKEN` |

DigitalOcean does not support `spot: true` in the same way GCP supports preemptible instances. The DigitalOcean provider returns an error for Spot requests.

## Local Hooks

Hooks run shell commands **on your local machine** at lifecycle boundaries. This
is different from `startup_commands`, which run **on the VM** after provisioning.
The common case is building artifacts locally before they are synced to the VM:

```yaml
hooks:
  pre_up:
    - npm ci
    - npm run build
  post_down:
    - ./scripts/cleanup-local.sh
```

Available hooks:

| Hook | Runs |
| --- | --- |
| `pre_up` | locally, before `up` creates any cloud resource or syncs |
| `post_up` | locally, after `up` succeeds |
| `pre_deploy` | locally, before `deploy` syncs anything (e.g. build assets) |
| `post_deploy` | locally, after `deploy` succeeds |
| `pre_down` | locally, before `down` tears the VM down |
| `post_down` | locally, after `down` completes |
| `pre_destroy` | locally, before `destroy` deletes anything |
| `post_destroy` | locally, after `destroy` completes |

Behavior:

- **`pre_*` hooks gate the operation**: if one exits non-zero, the operation is
  aborted and nothing is created/destroyed.
- **`post_*` hooks are best-effort**: a failure is logged but does not fail the
  command (the operation already succeeded).
- `destroy` fires only `pre_destroy`/`post_destroy`, never the `down` hooks, even
  though it tears the VM down internally.
- Each command runs through `sh -c` (so pipes and `&&` work) from the project's
  `sync_dir` (or the current directory if unset), inheriting your environment
  plus `SERVERKU_PROJECT`, `SERVERKU_PROVIDER`, and `SERVERKU_IP` (when known).

> **Security:** hooks execute arbitrary commands from the project config on your
> machine. Only run configs you trust. Hooks require a Unix `sh`.

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

- **DigitalOcean and GCP are both supported.** The domain's apex zone must
  already be managed by the provider — delegated to DigitalOcean, or a Cloud DNS
  managed zone in your GCP project. `serverku` matches each FQDN to the longest
  managed zone and writes the record (no zone name to configure).
- Record changes are idempotent: a record already pointing at the IP is left
  unchanged.
- A DNS write failure aborts `up` and tears down the VM (persistent storage is
  preserved).
- Freshly created records may take time to propagate; Caddy retries certificate
  issuance, but a low `ttl` helps during initial setup.

## Backups

`serverku backup <project>` snapshots the project's persistent disk via the cloud
provider. The VM may be up or down — only the disk is required.

```bash
serverku backup myapp
# Snapshot created: serverku-myapp-20260621-120000 (id: ...) from disk serverku-myapp-data
serverku backup myapp --name pre-migration
```

Notes:

- A snapshot name is generated as `serverku-<project>-<timestamp>` unless you pass
  `--name`.
- Snapshots are crash-consistent (taken live); for application-consistent backups,
  quiesce or stop the workload first (e.g. `serverku down`, then `backup`).
- The project must have `storage.enabled` and an existing disk (run `serverku up`
  at least once).

### Restoring

`serverku restore <project> <snapshot>` creates a new disk from a snapshot
(by name or ID, as printed by `backup`) and points the project at it:

```bash
serverku down myapp                       # a disk can't be swapped under a running VM
serverku restore myapp serverku-myapp-20260712-120000
serverku up myapp                         # attaches the restored disk
```

Notes:

- The project must be **stopped** first.
- The previous disk is **kept by default** so you can roll back; pass
  `--delete-old` to remove it (and stop paying for it) once you trust the
  restore.
- `--name` sets the restored disk's name (default
  `serverku-<project>-data-<timestamp>`).

## Costs

serverku is built to save budget, so it treats cost figures as
accuracy-sensitive. Two sources are used, and the display always tells you
which one you're looking at:

- **Live provider prices** — shown as-is (e.g. `$0.0089/hr`).
  - DigitalOcean: hourly rates from the `/v2/sizes` API (the prices DO
    itself bills by), plus real account **month-to-date usage** from the
    balance API, shown in `status`.
  - GCP: rates derived from the Cloud Billing Catalog API (core-hours +
    RAM GiB-hours per region, spot-aware). Requires the Cloud Billing API
    to be enabled; supports the e2/n1/n2/n2d families.
- **Offline table estimates** — the fallback when a live lookup is
  unavailable, always marked with `~`/`est.` (e.g. `~$0.0089/hr est.`).
  An estimate is never presented as a bill.

While a project runs, `status` and `list` show the **accrued session
cost** — what this VM has actually cost since `up`:

```text
Uptime:    7h30m12s
Session:   $0.07 ($0.0089/hr)
```

Figures cover VM compute (and storage monthly estimates); they exclude
taxes, bandwidth, and snapshots.

## Notifications

Set up a channel with the guided wizard — it connects the channel and
**verifies it end to end by sending a real test notification from your
machine** before saving anything:

```bash
serverku notify setup myapp   # pick ntfy or Telegram, connect, test, confirm
serverku notify test myapp    # re-send a test to every configured channel
```

**ntfy (recommended):** account-less publish/subscribe push where the
project's randomly generated topic name is the only credential. `init`
generates a topic automatically; `serverku ntfy myapp` shows it with
subscribe instructions (`--test` sends a test message). Subscribe once in
the ntfy app (iPhone/Android, free) or open `https://ntfy.sh/<topic>` in a
browser. The public ntfy.sh server is free; self-host with
`notifications.ntfy.server` if you prefer (iOS push then relays its wake-up
signal through ntfy.sh — an Apple/APNs constraint).

**Telegram:** the wizard verifies your bot token against the Telegram API,
then auto-discovers your `chat_id` — you just send your bot one message —
and finishes with a test send. No manual `getUpdates` spelunking.

## Still-Running Heartbeat

The most expensive VM is the one you forgot. With `heartbeat_hours` set
(on `ntfy` and/or `telegram`), provisioning installs a systemd timer
**on the VM** that pings you every N hours for as long as the VM runs:

```text
serverku: myapp still running -- up 7h30m, $0.07 so far ($0.0089/hr).
Stop with: serverku down myapp
```

```yaml
notifications:
  ntfy:
    topic: serverku-myapp-8d17e4e4
    heartbeat_hours: 6
```

Why on the VM instead of your machine?

- It keeps reminding you **while your laptop is off** — exactly when VMs
  get forgotten.
- It dies with the VM, so it can never false-alarm after `down`/`destroy`.
- The accrued cost in the message uses the same honest provenance rules as
  the CLI (live prices plain, estimates marked `est.`).

**Channel security trade-off** — the heartbeat must place its sending
credential on the VM (root-only, 0700):

| Channel | What lands on the VM | Worst case if the VM is compromised |
| --- | --- | --- |
| `ntfy` (recommended) | the random topic name — tied to no account | spam to that one topic; rotate by generating a new topic |
| `telegram` | your bot token | spam as your bot, read messages sent to the bot; revoke via BotFather |

If both channels have `heartbeat_hours`, the smaller interval wins and both
get pinged.

## Local Development

A Makefile wraps the common tasks (`make help` lists them all):

```bash
make build       # build the CLI binary
make test        # run all tests
make test-race   # tests with the race detector
make lint        # golangci-lint
make check       # fmt + vet + test (run before pushing)
```

Or with plain Go — build:

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

Local tests cover config, orchestration logic, pricing estimates, notification composition, provider behavior (via fake API endpoints), and end-to-end CLI flows. CI runs build, tests, and golangci-lint on every push and pull request.

See [`docs/development.md`](docs/development.md) for the contributor guide and offline verification notes.

## Safety Notes

- `serverku up` can create billable cloud resources.
- `serverku down` destroys the VM but keeps persistent storage.
- `serverku destroy` deletes project resources and should be treated as irreversible.
- Use small instances and throwaway projects for testing.
- Always confirm resources are cleaned up in your cloud provider console after experiments.

## Roadmap

- [ ] **Hetzner Cloud provider** — the most-requested next cloud. The
  `CloudProvider` interface is small and well-trodden; see
  [`docs/development.md`](docs/development.md#adding-a-cloud-provider) if
  you want to contribute it.
- [ ] `vm.max_uptime` kill switch — the heartbeat tells you a VM is still
  running; this one would act on it.
- [ ] Scheduled/automatic backups (the `backup`/`restore` primitives exist).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) — bug fixes, tutorials, and new
providers are all welcome. Security reports: [SECURITY.md](SECURITY.md).

## License

MIT
