# Contributing to serverku

Thanks for your interest! serverku is a small, focused tool: cloud VMs as
disposable runtime for Docker Compose projects. Contributions that sharpen
that focus are very welcome.

## Getting started

```bash
git clone https://github.com/jufianto/serverku.git
cd serverku
make build && make test
```

See [`docs/development.md`](docs/development.md) for the full development
guide — including how to test everything offline (no cloud account needed)
and how to exercise a dev build against a throwaway `--config-dir`.

## What makes a good contribution

- **Bug fixes** with a failing test that the fix turns green.
- **New cloud providers** — the most-wanted is Hetzner. The interface is
  small; `docs/development.md` has a step-by-step walkthrough, and the
  existing GCP/DigitalOcean packages show the fake-endpoint test pattern.
- **Docs and tutorials** — `docs/` has blog-style tutorials; more real-app
  walkthroughs are welcome.

## Ground rules

- `make check` (fmt + vet + test) must pass; CI also runs golangci-lint
  and the race detector.
- New behavior needs tests. The codebase tests everything offline — mock
  providers for orchestrator logic, `httptest` fakes for provider APIs,
  compiled-binary e2e for CLI behavior. Follow those patterns.
- **Cost figures are accuracy-sensitive**: live API prices render plain,
  offline estimates must always carry an `est.` marker. Never present an
  estimate as a bill.
- Cloud credentials must never be written to the VM or to disk by
  serverku. The only exception is the opt-in heartbeat channel credential,
  which is documented in the README with its threat model.
- Keep commits purpose-separated with clear conventional-style messages
  (`feat:`, `fix:`, `docs:`, `chore:`, `test:`).

## Pull requests

- One PR per coherent piece of work; multiple commits inside are fine.
- Describe what you verified and how (test output, real-cloud smoke test,
  etc.).
- If you change user-facing behavior, update the README (and tutorials if
  they demonstrate the old behavior).

## Questions / ideas

Open a GitHub issue — including "would you accept X?" design questions
before you invest time in building X.
