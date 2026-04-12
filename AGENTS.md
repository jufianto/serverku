# serverku Agent Notes

## Entry Points
- CLI entrypoint is `cmd/serverku/main.go`.
- User-facing commands live in `cmd/serverku/init.go`, `cmd/serverku/lifecycle.go`, and `cmd/serverku/status.go`.
- Core lifecycle logic is in `internal/orchestrator/orchestrator.go`; cloud implementations plug into `internal/provider/provider.go`.

## Current Reality
- The only implemented cloud provider is GCP. `digitalocean` is accepted by config validation, but `newProviderFactory()` in `cmd/serverku/lifecycle.go` returns `"DigitalOcean provider not yet implemented"`.
- `internal/legacy/` is reference-only and not used by the CLI.
- `up` does not provision Docker or deploy Compose yet. `compose_file` and `startup_commands` exist in config/examples, but there are no Go code references to them.
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
- There is no Makefile, task runner, linter config, formatter config, or CI workflow in this repo. Prefer targeted `go test` for touched packages, then `go test ./...`, then a build if CLI code changed.

## GCP Notes
- GCP provider construction is in `internal/provider/gcp/gcp.go` and uses `compute.NewService(ctx)`, so local Application Default Credentials must already work for real provider calls.
- Provider operations use GCP instance and disk names for most follow-up API calls, even when state also stores numeric IDs. Preserve both when changing lifecycle code.

## OpenSpec
- This repo uses [OpenSpec](https://openspec.dev/) for spec-driven planning of medium-to-large features.
- Specs live in `openspec/specs/`; active change proposals in `openspec/changes/`.
- Slash commands are in `.opencode/commands/`: `/opsx:propose`, `/opsx:apply`, `/opsx:explore`, `/opsx:archive`.
- Use OpenSpec for multi-file features (new providers, provisioner, notifications). Skip it for small fixes or single-file changes.
