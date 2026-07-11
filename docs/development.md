# Development Guide

How to build, test, and verify serverku without touching real cloud
resources.

## Build and test

```bash
make build       # build the CLI binary
make test        # run all tests
make test-race   # tests with the race detector
make lint        # golangci-lint
make check       # fmt + vet + test (run before pushing)
```

Go 1.23+. CI runs build, tests, and golangci-lint on every push and PR.

## Test against a throwaway config dir

Never point a dev build at your real `~/.serverku/`:

```bash
tmpdir="$(mktemp -d)"
./serverku --config-dir "$tmpdir" init demo --non-interactive \
  --provider digitalocean --region sgp1 --size s-1vcpu-1gb --no-storage
./serverku --config-dir "$tmpdir" list
```

## What is covered offline

The test suite runs entirely without cloud credentials:

- **Config/store** — YAML round-trips, validation rules, state files.
- **Orchestrator** — full up/down/destroy/backup flows against a mock
  provider and provisioner, including failure paths (disk attach fails,
  provisioning fails, firewall fails) and hook gating.
- **Providers** — GCP and DigitalOcean exercised against fake HTTP API
  endpoints (`httptest`), including pricing catalogs, snapshots, DNS,
  and error mapping.
- **Notifiers** — Slack/Telegram/ntfy against fake endpoints, including
  Telegram chat discovery.
- **Provisioner scripts** — generated shell (mount, compose, heartbeat)
  asserted for content; heartbeat also syntax-checked with `sh -n`.
- **CLI end-to-end** — the compiled binary is executed against a temp
  config dir (init flags, list, ntfy, notify test, error messages).

What still needs real credentials: actual `up`/`down` against GCP or
DigitalOcean. Use small instances and a throwaway project, and confirm
cleanup in the provider console afterwards.

## Architecture notes

See [`01-architecture.md`](01-architecture.md),
[`04-data-persistence-strategy.md`](04-data-persistence-strategy.md), and
the repo-level `CLAUDE.md`/`AGENTS.md` for the component map. The core
rule: commands wire concrete implementations, `internal/orchestrator`
stays provider-agnostic, and optional provider capabilities (DNS,
firewall, pricing) are separate interfaces detected by type assertion.

## Adding a cloud provider

1. Implement `provider.CloudProvider` (`internal/provider/provider.go`)
   in a new package under `internal/provider/<name>/`.
2. Optionally implement the capability interfaces: `DNSManager`,
   `FirewallManager`, `PriceCatalog`, `UsageReporter`.
3. Add a case to `newProviderFactory()` in `cmd/serverku/lifecycle.go`.
4. Add pricing table entries in `internal/pricing/pricing.go` as the
   offline fallback.
5. Test against a fake HTTP endpoint like the existing providers do.

Hetzner is the most-wanted next provider — see the README roadmap.
