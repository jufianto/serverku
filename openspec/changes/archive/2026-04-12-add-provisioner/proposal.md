## Why

The orchestrator's `Up()` flow creates a VM, attaches storage, and records an IP -- but it never installs software on the VM. The comment at `orchestrator.go:47` says explicitly: "It does NOT provision the VM -- that's the provisioner's job." Without a provisioner, `serverku up` delivers a bare Ubuntu VM with no Docker, no mounted data disk, and no deployed containers. Users have to SSH in manually and set everything up, which defeats the tool's purpose of one-command deployment.

## What Changes

- Add a new `internal/provisioner/` package implementing SSH-based server provisioning.
- The provisioner will: wait for SSH readiness, install Docker + Docker Compose, mount the persistent disk (if storage is enabled), transfer the user's `docker-compose.yml`, and run `docker compose up -d`.
- A `Teardown` method will cleanly stop containers and unmount the disk before `Down()` destroys the VM.
- The orchestrator's `Up()` will call `provisioner.Provision()` after getting the external IP (between current steps 8 and 9).
- The orchestrator's `Down()` will call `provisioner.Teardown()` before detaching the disk (before current step 3).
- The `compose_file` and `startup_commands` fields in `ProjectConfig` will become functional -- they are currently defined in the struct but never referenced by any Go code.

## Capabilities

### New Capabilities
- `ssh-provisioner`: SSH client wrapper with retry/backoff, Docker installation, disk formatting/mounting, compose file transfer, container lifecycle management, and teardown logic.

### Modified Capabilities
<!-- No existing specs to modify -- openspec/specs/ is empty. -->

## Impact

- **New package**: `internal/provisioner/` (new files: `provisioner.go`, `ssh.go`, `scripts.go`, `provisioner_test.go`)
- **Modified file**: `internal/orchestrator/orchestrator.go` -- `Up()` and `Down()` gain provisioner calls; `Orchestrator` struct gains a `provisioner` field.
- **New dependency**: `golang.org/x/crypto/ssh` is already in `go.mod` (used by `internal/config/keys.go`). No new external dependencies required.
- **Config fields activated**: `ProjectConfig.ComposeFile` and `ProjectConfig.StartupCommands` will be read and used by the provisioner.
- **SSH user convention**: GCP VMs use the username embedded in the SSH key metadata (serverku sets `"serverku:<pubkey>"`, so the SSH user is `serverku`). The provisioner needs to connect as that user and use `sudo` for privileged commands.
- **Disk device path**: GCP attaches the data disk as `/dev/disk/by-id/google-<disk-name>`. The provisioner must use this stable path rather than `/dev/sdb` which can shift.
