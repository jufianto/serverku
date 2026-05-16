## Context

`serverku` currently only supports transferring the `docker-compose.yml` file to the remote VM via an SSH heredoc block. This is insufficient for real-world projects that require `.env` files, configuration files (like `nginx.conf`), or application source code. We need a robust mechanism to synchronize an entire local directory to the VM before starting the Docker Compose stack.

## Goals / Non-Goals

**Goals:**
- Add `SyncDir` support to the project configuration.
- Sync the configured local directory to the remote VM's deployment directory (e.g., `/data/` if storage is enabled, or `/home/serverku/` if not).
- Support standard `.gitignore` or `.serverkuignore` exclusions if possible, though initially, we might just sync everything in the target directory.

**Non-Goals:**
- We will not build a custom sync protocol in Go (like a custom rsync implementation). We will rely on existing system binaries (`rsync` or `scp`).

## Decisions

**1. Tool Selection (`scp` vs `rsync`):**
We will prefer `rsync` if available, and fallback to `scp` if not.
- *Rationale*: `rsync` is much faster for delta syncs (e.g., when the user runs `serverku up` again after changing a few files). It also has better support for exclusions. However, `scp` is more universally installed, so providing it as a fallback is a good idea, or we could just mandate `rsync`. Let's mandate `rsync` for simplicity and performance, as it's standard on almost all dev machines (macOS/Linux). If not found, we return an error.

**2. Where to Sync:**
The `Provisioner` will sync the local directory to the remote compose directory before running `docker compose up -d`.

**3. Ignoring Files:**
We will pass standard exclude flags to `rsync` (e.g., `--exclude=.git`, `--exclude=node_modules`). Eventually, we can support a `.serverkuignore` file.

## Risks / Trade-offs

- **[Risk] `rsync` not installed on user's machine** → *Mitigation*: Fail fast with a clear message instructing them to install it.
- **[Risk] Syncing large directories (e.g., local `node_modules`)** → *Mitigation*: We will hardcode common exclusions (`.git`, `node_modules`, `vendor`) into the `rsync` command by default to prevent massive unnecessary uploads.
