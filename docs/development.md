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

## Cloud action logs

Detailed cloud API logs are hidden by default for DigitalOcean and GCP. Enable
reads, writes, polling, HTTP results, and request duration for one command:

```bash
serverku status kuma --debug
```

To enable them persistently, set `debug: true` in `~/.serverku/config.yaml`
(or `<config-dir>/config.yaml` when using `--config-dir`). A missing file or
`debug: false` keeps them hidden. `--verbose` also enables API logs, and an
explicit `--debug=false` overrides the file and `--verbose`.

Fresh VMs can become active before SSH accepts connections. During `up` and
`deploy`, Serverku shows a waiting message and confirms when SSH connects.
Individual connection retries are diagnostics shown only when debug is enabled;
the final timeout or connection failure remains visible in normal output.

Each new project uses its own generated SSH key. DigitalOcean registers it as
`serverku-<project>`; `status` finds it by fingerprint. `down` retains keys and
`destroy` removes generated local keys and account keys created by that project.
Custom, legacy shared, externally registered, and still-referenced keys are
retained. A failed key lookup shows `unknown`; `none` means the key was confirmed
unregistered. See [Project SSH keys](../guides/project-ssh-keys.md).

Normal status output, progress messages, warnings, and errors stay visible. Lifecycle
and provisioning messages explain actions such as reusing/registering an SSH
key, injecting it into a VM, creating storage, transferring Compose, and starting
containers. Request headers, query strings, and bodies are excluded from API
logs. Remote command output can still contain application-specific information.

Logs are terminal output, not a persistent audit file. To save a run:

```bash
serverku up kuma --debug 2>&1 | tee serverku-kuma.log
```

On provisioning failure, serverku attempts to delete the VM and preserves any
persistent disk. Successful cleanup clears the saved VM ID, name, and IP while
retaining the failure reason. Failed cleanup reports the deletion error and
keeps the VM tracked for inspection.

## What is covered offline

For a local GCP emulator setup and its current coverage limits, see
[`floci-gcp.md`](floci-gcp.md).

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

## Versioning and releases

serverku follows [semantic versioning](https://semver.org/) with a `v` prefix
(`v0.1.0`). While the API and config format may still change, the major version
stays at `0`; breaking changes bump the minor.

`serverku --version` reports build metadata:

- **Release binaries** (built by goreleaser on a tag) show the clean tag plus
  commit and date, e.g. `v0.1.0 (a1b2c3d4e5f6, 2026-07-15T10:00:00Z)`.
- **Local `go build`** shows `dev` plus the git revision and a `-dirty` marker
  when the working tree has uncommitted changes — Go embeds this VCS info
  automatically, so no build flags are needed to identify a dev binary.
- **`go install ...@vX.Y.Z`** shows the module version.

The `version`/`commit`/`date` variables live in `cmd/serverku/main.go`;
goreleaser stamps them via `-ldflags`, and `resolveVersion()` fills any gaps
from the embedded build info.

### Cutting a release

Releases are fully automated by `.github/workflows/release.yml` — pushing a
`v*` tag runs the tests and then goreleaser (`.goreleaser.yaml`), which builds
cross-platform binaries, checksums, and a GitHub Release.

```bash
# 1. make sure master is green and up to date
git checkout master && git pull

# 2. tag (annotated) and push the tag
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

Do a dry run without publishing anything with `goreleaser release --snapshot
--clean` (requires the goreleaser CLI locally).

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
