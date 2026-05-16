# Internal README

This document is a working reference for the features currently implemented in `serverku` and how to verify them. It separates checks that can run locally without cloud access from checks that require real GCP/DigitalOcean credentials.

## Current Feature Inventory

### Core CLI

- `serverku init <project>` creates project configuration and SSH keys.
- `serverku init <project>` runs an interactive `huh` wizard when used in a terminal.
- `serverku init <project> --non-interactive` skips prompts and creates config from flags/defaults.
- `serverku up <project>` creates infrastructure, attaches storage, provisions Docker, syncs/deploys Compose, and reports IP/resource IDs.
- `serverku down <project>` tears down the VM while preserving persistent storage.
- `serverku destroy <project>` deletes VM, storage, state, and config.
- `serverku status <project>` shows project state and reconciles with the provider.
- `serverku list` lists projects, state, IP, storage, and estimated cost.
- `serverku ssh <project>` opens an interactive SSH session to the running VM.
- `serverku logs <project>` streams `docker compose logs -f` from the VM.
- `serverku tunnel <project> <local-port>:<remote-port>` creates an SSH port-forwarding tunnel.

### Providers

- GCP provider via Compute Engine SDK.
- DigitalOcean provider via `github.com/digitalocean/godo`.
- DigitalOcean authentication uses `DIGITALOCEAN_TOKEN`.
- DigitalOcean Spot is rejected because DO does not provide Spot VMs in the same way.

### Provisioning

- Installs Docker on Ubuntu VMs.
- Mounts persistent storage when enabled.
- Writes `docker-compose.yml` to the remote compose directory.
- Syncs full project artifacts with `rsync` when `sync_dir` is configured.
- Runs configured startup commands.
- Runs teardown script before VM destruction where possible.

### Caddyku Integration

- Supports `router` config with domains, service, and upstream values.
- Installs the `caddyku` binary on the remote VM.
- Runs `caddyku init` and starts the global proxy stack.
- Runs `caddyku init-app` for each configured domain.

### Notifications

- `NoopNotifier` when notifications are not configured.
- `MultiNotifier` to send to multiple channels.
- Slack incoming webhook notifier.
- Telegram Bot API notifier.
- Error notifications from orchestrator error paths.

### Cost Estimation

- `internal/pricing` contains approximate VM and storage pricing.
- `serverku up` prints estimated hourly/monthly running costs.
- `serverku list` prints compact estimated cost per project.
- Pricing is approximate and intentionally does not include bandwidth, snapshots, reserved discounts, taxes, or provider-specific promotions.

## Configuration Shape

Example project YAML:

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

compose_file: ./docker-compose.yml
sync_dir: ./myapp

router:
  enabled: true
  domains:
    - domain: myapp.example.com
      service: web
      upstream: web:3000

notifications:
  slack:
    webhook_url: "https://hooks.slack.com/services/..."
  telegram:
    bot_token: "123456:ABC"
    chat_id: "123456789"
```

## Local Verification Without Cloud Access

These checks should work without GCP or DigitalOcean credentials.

### Build And Unit Tests

```bash
go build -o serverku ./cmd/serverku
go test ./...
```

Expected result:

- Build succeeds.
- All package tests pass.

### CLI Command Registration

```bash
./serverku help
```

Expected commands include:

- `init`
- `up`
- `down`
- `destroy`
- `status`
- `list`
- `ssh`
- `logs`
- `tunnel`

### Non-Interactive Init

Use a temporary config directory so the test does not touch `~/.serverku`.

```bash
tmpdir="$(mktemp -d)"
./serverku --config-dir "$tmpdir" init demo --non-interactive --provider digitalocean --region sgp1 --vm-size s-1vcpu-1gb --no-storage
ls "$tmpdir/projects"
```

Expected result:

- `demo.yaml` exists under `$tmpdir/projects`.
- SSH keys exist under `$tmpdir/keys`.

### List And Cost Estimation

After creating the demo project above:

```bash
./serverku --config-dir "$tmpdir" list
```

Expected result:

- Output includes `EST. COST` column.
- DigitalOcean `s-1vcpu-1gb` shows an approximate VM/storage cost string.

### Status Without Cloud Credentials

```bash
./serverku --config-dir "$tmpdir" status demo
```

Expected result:

- Shows local project state.
- If provider verification cannot happen due to missing credentials, status reconciliation may warn or fail depending on provider setup.

### SSH / Logs / Tunnel Not Running Errors

With the demo project stopped:

```bash
./serverku --config-dir "$tmpdir" ssh demo
./serverku --config-dir "$tmpdir" logs demo
./serverku --config-dir "$tmpdir" tunnel demo 5432:5432
```

Expected result:

- Each command returns a clear error that the project is not running or has no external IP.

### Tunnel Port Mapping Validation

```bash
./serverku --config-dir "$tmpdir" tunnel demo bad
./serverku --config-dir "$tmpdir" tunnel demo 0:5432
./serverku --config-dir "$tmpdir" tunnel demo 5432:70000
```

Expected result:

- Each command returns a validation error before attempting SSH.

### Pricing Unit Tests

```bash
go test ./internal/pricing
```

Expected result:

- Known GCP and DigitalOcean sizes return known prices.
- Unknown sizes report unavailable VM pricing.
- Disabled storage reports zero storage cost.

### Notification Unit Tests

```bash
go test ./internal/notify
```

Expected result:

- MultiNotifier calls all configured notifiers when successful.
- MultiNotifier returns the first notifier error.

### DigitalOcean Provider Unit Tests

```bash
go test ./internal/provider/digitalocean
```

Expected result:

- Missing `DIGITALOCEAN_TOKEN` is detected.
- Spot VM requests are rejected.
- Public IPv4 extraction works.

## What We Can Test Better Without Cloud Access

Yes, we can add more offline tests, but some require small refactors.

Recommended future test improvements:

- Extract SSH command construction for `ssh`, `logs`, and `tunnel` into small helper functions and unit-test the generated arguments.
- Extract `rsync` command construction from `SSHProvisioner` and unit-test source, destination, excludes, and SSH options.
- Extract Caddyku script generation and unit-test the generated commands for configured domains.
- Add HTTP server tests for Slack and Telegram notifiers to verify JSON payloads and timeout/error handling without real Slack/Telegram accounts.
- Add provider interface mocks for orchestrator tests that cover `up`, `down`, and `destroy` across GCP/DO-like behavior without real cloud calls.
- Add config serialization tests for `sync_dir`, `router`, and `notifications` YAML fields.

## Integration Tests That Require Real Cloud Access

These tests create real resources and may cost money. Use a throwaway project and small instances.

### DigitalOcean End-To-End

Prerequisites:

- `DIGITALOCEAN_TOKEN` with read/write permissions.
- `rsync` and `ssh` installed locally.
- A test domain only if testing `router`/Caddyku.

Suggested flow:

```bash
export DIGITALOCEAN_TOKEN="..."
tmpdir="$(mktemp -d)"

./serverku --config-dir "$tmpdir" init do-demo --non-interactive --provider digitalocean --region sgp1 --vm-size s-1vcpu-1gb --no-storage
./serverku --config-dir "$tmpdir" up do-demo
./serverku --config-dir "$tmpdir" status do-demo
./serverku --config-dir "$tmpdir" logs do-demo
./serverku --config-dir "$tmpdir" tunnel do-demo 8080:80
./serverku --config-dir "$tmpdir" down do-demo --force
./serverku --config-dir "$tmpdir" destroy do-demo --force
```

Expected result:

- Droplet is created and reaches running state.
- SSH key is usable.
- `down` removes VM while preserving storage if storage was enabled.
- `destroy` removes all resources.

### GCP End-To-End

Prerequisites:

- Application Default Credentials configured locally.
- GCP project with Compute Engine API enabled.
- Permissions to create instances, disks, firewall/IP resources as required by the provider.

Suggested flow:

```bash
tmpdir="$(mktemp -d)"

./serverku --config-dir "$tmpdir" init gcp-demo --non-interactive --provider gcp --project-id YOUR_GCP_PROJECT --region asia-southeast1 --zone asia-southeast1-b --vm-size e2-micro --no-storage
./serverku --config-dir "$tmpdir" up gcp-demo
./serverku --config-dir "$tmpdir" status gcp-demo
./serverku --config-dir "$tmpdir" down gcp-demo --force
./serverku --config-dir "$tmpdir" destroy gcp-demo --force
```

Expected result:

- Instance is created and becomes SSH-ready.
- Status reconciliation works.
- Resources are cleaned up.

## Real-World Feature Test Matrix

| Feature | Local test possible? | Current automated coverage | Needs cloud for confidence? |
| --- | --- | --- | --- |
| Config validation | Yes | Yes | No |
| Store layout | Yes | Yes | No |
| Interactive init | Partial | Build only | No, but manual TTY test recommended |
| Non-interactive init | Yes | Manual command | No |
| GCP provider | Partial with mocks | Limited | Yes |
| DigitalOcean provider | Partial | Unit tests | Yes |
| Orchestrator lifecycle | Yes with mocks | Yes | Yes for final E2E |
| SSH provisioning scripts | Partial | Some tests | Yes for full VM setup |
| Artifact sync | Yes after helper extraction | Build only | Yes for full remote sync |
| Caddyku integration | Yes after helper extraction | Build only | Yes for real HTTPS |
| Notifications | Yes | MultiNotifier tests | Optional for real channel delivery |
| SSH command | Yes after helper extraction | Build only | Yes for real shell |
| Logs command | Yes after helper extraction | Build only | Yes for real log streaming |
| Tunnel command | Yes after helper extraction | Build only | Yes for real forwarding |
| Cost estimation | Yes | Unit tests | No |

## Recommended Next Testing Work

Before running real cloud tests, improve offline confidence with these changes:

1. Add command-construction unit tests for `ssh`, `logs`, and `tunnel`.
2. Add script-generation tests for Caddyku and Docker Compose provisioning.
3. Add `httptest` coverage for Slack and Telegram payloads.
4. Add fake provider tests for DigitalOcean lifecycle mapping if the godo client can be wrapped behind a small interface.
5. Add one guarded integration test file using build tags, e.g. `//go:build integration`, so real cloud tests only run with `go test -tags=integration`.

## Safe Test Commands Summary

```bash
go build -o serverku ./cmd/serverku
go test ./...
go test ./internal/pricing
go test ./internal/notify
go test ./internal/provider/digitalocean
./serverku help
```

These commands do not create cloud resources.
