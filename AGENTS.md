# serverku Agent Notes

## Entry Points
- CLI entrypoint is `cmd/serverku/main.go`.
- User-facing commands live in `cmd/serverku/`: `init.go`, `lifecycle.go` (up/down/destroy), `status.go`, `logs.go`, `ssh.go`, `tunnel.go`.
- Core lifecycle logic is in `internal/orchestrator/orchestrator.go`; cloud implementations plug into `internal/provider/provider.go`.

## Current Reality
- Both GCP and DigitalOcean providers are implemented. `newProviderFactory()` in `cmd/serverku/lifecycle.go` selects by `cfg.Provider` (`gcp`, `digitalocean`). DigitalOcean rejects `spot: true` (no preemptible equivalent).
- `up` provisions over SSH via `internal/provisioner/`: installs Docker, mounts storage, syncs the project with `rsync`, embeds the Compose file into a remote script, optionally installs/inits Caddyku, then runs `docker compose up -d`.
- `status` reconciles tracked state against the provider. If a SPOT VM was terminated externally, local state is rewritten back to `stopped` and VM/IP fields are cleared.

## Config And State
- Default working data lives under `~/.serverku/`; pass `--config-dir` to point the CLI at a different base dir.
- Store layout is fixed by `internal/config/store.go`.
- `projects/<name>.yaml`: user config.
- `state/<name>.json`: runtime state.
- `keys/serverku_rsa*`: generated SSH keypair.
- `serverku init` creates the project YAML and ensures the SSH keypair exists.
- Project names are stricter than many CLIs: lowercase letters, digits, and hyphens only; no leading/trailing hyphen.

## Commands
- Build: `go build -o serverku ./cmd/serverku`
- Run all tests: `go test ./...`
- Run one package: `go test ./internal/orchestrator`
- Run one test: `go test ./internal/orchestrator -run TestUpWithStorage`

## Verification
- A Makefile, golangci-lint config and GitHub CI are present. Run `make check` before a PR; prefer targeted `go test` for touched packages, then `go test ./...`, then a build if CLI code changed. CI also runs race tests and lint.

## Branding and review
- Read [the brand reference](docs/reference/brand.md), [serverku-brand skill](.claude/skills/serverku-brand/SKILL.md), [skill index](.claude/skills/README.md) and [review rules](docs/review.md) for graphical identity or future frontend changes.
- `brand/tokens.json` is the canonical palette. Run `make brand-check`; kit changes also require `make brand-verify` and `make brand-rebuild`. Preserve terminal appearance.

## GCP Notes
- GCP provider construction is in `internal/provider/gcp/gcp.go` and uses `compute.NewService(ctx)`, so local Application Default Credentials must already work for real provider calls.
- Provider operations use GCP instance and disk names for most follow-up API calls, even when state also stores numeric IDs. Preserve both when changing lifecycle code.
