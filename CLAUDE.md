# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`serverku` is a CLI that treats cloud VMs as disposable runtime for Docker Compose
projects. `up` creates a VM, attaches persistent block storage, provisions Docker,
syncs the project with `rsync`, and runs `docker compose up -d`; `down` destroys the
VM but keeps storage; `destroy` removes everything. The value proposition is paying
for compute only while in use while keeping data on storage between sessions.

## Commands

```bash
go build -o serverku ./cmd/serverku   # build the CLI
go test ./...                          # all tests
go test ./internal/orchestrator        # one package
go test ./internal/orchestrator -run TestUpWithStorage   # one test
```

There is no Makefile, linter config, formatter config, or CI workflow. Verification
order: targeted `go test` for touched packages → `go test ./...` → a build if CLI
code changed. Go 1.23+.

Test locally against a throwaway config dir to avoid touching `~/.serverku/`:

```bash
tmpdir="$(mktemp -d)"
./serverku --config-dir "$tmpdir" init demo --non-interactive \
  --provider digitalocean --region sgp1 --size s-1vcpu-1gb --no-storage
./serverku --config-dir "$tmpdir" list
```

## Architecture

The dependency flow is: **CLI command (`cmd/serverku/`) → `Orchestrator` →
`CloudProvider` + `Provisioner` + `Notifier`**. Commands wire concrete
implementations; the orchestrator is provider-agnostic.

- **`cmd/serverku/`** — Cobra commands (`init`, `lifecycle` for up/down/destroy,
  `status`, `logs`, `ssh`, `tunnel`, `backup`). `lifecycle.go` holds `newProviderFactory()`,
  which selects the provider by `cfg.Provider` (`gcp` or `digitalocean`), and
  `buildNotifier()`, which assembles a multi-notifier from config.
- **`internal/orchestrator/`** — `Up`/`Down`/`Status`/`Destroy`. Owns the full
  lifecycle sequence and all state transitions. Takes a `ProviderFactory` so a
  provider is constructed per project from config. `Status` reconciles tracked
  state against the live provider — e.g. a SPOT VM terminated externally is rewritten
  back to `stopped` with VM/IP fields cleared. Runs local lifecycle hooks via an
  injected `HookRunner` (`pre_*` gate the operation; `post_*` are best-effort).
  `Destroy` calls an internal `down(..., suppressHooks=true)` so it fires only its
  own hooks, not the `down` hooks.
- **`internal/hooks/`** — runs a project's local hook commands via `sh -c`
  (local machine, not the VM — distinct from `startup_commands`).
- **`internal/provider/`** — `CloudProvider` interface (`provider.go`) plus `gcp/`
  and `digitalocean/` implementations. **Both providers are implemented.** Add a new
  cloud by implementing the interface and adding a case in `newProviderFactory()`.
  DigitalOcean rejects `spot: true` (no equivalent to GCP preemptible).
  `DNSManager` is an *optional* capability interface — providers that support DNS
  automation implement `EnsureARecord`; the orchestrator detects it via a type
  assertion on the `CloudProvider` and `up` fails fast if `dns.enabled` on a
  provider that doesn't. Both GCP (Cloud DNS) and DigitalOcean implement it.
  `FirewallManager` is another optional capability (same type-assertion pattern):
  `up` ensures the project's firewall rule before creating the VM and `destroy`
  removes it best-effort. Only GCP implements it (DigitalOcean droplets are open
  by default). `SnapshotDisk` (part of `CloudProvider`) backs the `serverku
  backup` command.
- **`internal/provisioner/`** — runs provisioning over SSH: installs Docker, mounts
  storage, syncs the project, embeds the Compose file into a remote script, optionally
  installs/inits Caddyku, then `docker compose up -d`. `scripts.go` builds the remote
  shell scripts.
- **`internal/config/`** — `store.go` defines the fixed on-disk layout. Project
  config and runtime state are separate: `projects/<name>.yaml` (user config) vs
  `state/<name>.json` (runtime state). Provider operations key off VM/disk *names*
  for follow-up API calls even though numeric IDs are also stored — preserve both
  when editing lifecycle code.
- **`internal/pricing/`** — offline pricing tables plus the `Rate` type that
  carries price provenance (`Live` API price vs table estimate). Live prices
  come from providers implementing the optional `PriceCatalog` capability
  (DO `/v2/sizes`, GCP Billing Catalog SKUs); table fallbacks must always be
  displayed with an `est.` marker — never present an estimate as a real price.
  `UsageReporter` (DO only) reports real month-to-date account usage.
  `cmd/serverku/cost.go` holds the per-invocation `rateCache`.
- **`internal/notify/`** — Slack and Telegram notifiers behind a `Notifier` interface.

### Config and state on disk

Default base dir is `~/.serverku/` (override with `--config-dir`):

- `projects/<name>.yaml` — user config
- `state/<name>.json` — runtime state, reconciled by `status`
- `keys/serverku_rsa{,.pub}` — managed SSH keypair, created by `init`

Project names are validated strictly: lowercase letters, digits, and hyphens only;
no leading/trailing hyphen.

## GCP notes

GCP provider construction (`internal/provider/gcp/gcp.go`) uses `compute.NewService(ctx)`,
so local Application Default Credentials must already work for real provider calls.
DigitalOcean uses `DIGITALOCEAN_TOKEN`.

