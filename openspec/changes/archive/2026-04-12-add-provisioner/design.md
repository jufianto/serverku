## Context

serverku is a Go CLI that orchestrates cloud VMs on demand. Phases 1-6 are complete: the CLI, config store, GCP provider, and orchestrator all work. The orchestrator creates VMs and attaches persistent disks, but the VM is left bare -- no Docker, no mounted data disk, no deployed containers. The provisioner is Phase 7 in the implementation plan and is the critical gap between "VM exists" and "application is running."

The SSH key infrastructure already exists (`internal/config/keys.go` generates a 4096-bit RSA keypair, stores it at `~/.serverku/keys/serverku_rsa*`). The public key is injected into VM metadata during creation. The `golang.org/x/crypto/ssh` package is already in `go.mod`.

The orchestrator currently has two explicit hooks for the provisioner:
- `Up()` comment at line 47: "It does NOT provision the VM -- that's the provisioner's job"
- `Down()` comment block mentions `provisioner.Teardown()` but never calls it

The `ProjectConfig` struct has `ComposeFile` and `StartupCommands` fields that are defined but never read by any Go code.

## Goals / Non-Goals

**Goals:**
- Implement a `Provisioner` interface and a concrete SSH-based implementation in `internal/provisioner/`.
- Install Docker and Docker Compose on a freshly created Ubuntu VM via SSH.
- Format (if new) and mount the persistent data disk when storage is enabled.
- Transfer the user's `docker-compose.yml` to the VM and run `docker compose up -d`.
- Execute optional `startup_commands` from project config after compose is running.
- Cleanly tear down containers and unmount the disk before VM destruction.
- Wire the provisioner into the orchestrator's `Up()` and `Down()` flows.
- Achieve full unit test coverage using a mock SSH layer.

**Non-Goals:**
- Supporting non-Ubuntu images (hardcode Ubuntu assumptions for now).
- Supporting non-SSH provisioning methods (e.g., cloud-init, GCP startup scripts).
- Docker registry authentication (images must be public or pre-authenticated).
- Managing `.env` files or secrets injection (users handle this in their compose file or mount path).
- Provisioning anything beyond Docker -- no package managers, databases, etc.
- DigitalOcean-specific provisioning differences (DO support is Phase 10; the interface will accommodate it, but only GCP logic is implemented now).

## Decisions

### 1. Provisioner as an interface, not a concrete dependency

The orchestrator will depend on a `Provisioner` interface, similar to how it uses `ProviderFactory` today. This lets tests mock the provisioner without real SSH.

```go
type Provisioner interface {
    Provision(ctx context.Context, opts ProvisionOpts) error
    Teardown(ctx context.Context, opts TeardownOpts) error
}
```

**Alternative considered**: Direct SSH calls in the orchestrator. Rejected because it would make the orchestrator untestable without an SSH server and would violate the existing layered architecture (orchestrator -> provider, orchestrator -> provisioner).

### 2. SSH connection via `golang.org/x/crypto/ssh`

Use the `ssh` package already in `go.mod`. The provisioner will:
1. Parse the private key from `~/.serverku/keys/serverku_rsa`.
2. Connect as user `serverku` (GCP metadata format `serverku:<pubkey>` creates this user).
3. Retry with exponential backoff: 5s, 10s, 20s, 40s... up to 5 minutes total.
4. Execute commands via `session.CombinedOutput()` for simplicity.

**Alternative considered**: `os/exec` shelling out to the system `ssh` binary. Rejected because it introduces a host dependency and is harder to test.

### 3. Commands executed as shell scripts, not individual SSH sessions

Each provisioning stage (install Docker, mount disk, deploy compose) will be a multi-line shell script executed in a single SSH session. This avoids the overhead and failure modes of opening/closing many sessions.

The scripts will be Go string constants in `internal/provisioner/scripts.go`, with `fmt.Sprintf` for parameterization (mount path, disk device, compose content).

**Alternative considered**: Running each command in its own session. Rejected because of connection overhead and because multi-step operations (e.g., format-then-mount) need to share a session to fail atomically.

### 4. Disk device path: use `/dev/disk/by-id/google-<disk-name>`

GCP exposes attached disks with stable symlinks under `/dev/disk/by-id/`. The provisioner will use `google-<disk-name>` (e.g., `google-serverku-myproject-data`) rather than `/dev/sdb` which can shift if multiple disks are present.

### 5. Disk formatting: check with `blkid` before `mkfs`

The provisioner MUST NOT format an existing disk. It will run `blkid <device>` first; only if that returns non-zero (no filesystem detected) will it run `mkfs.ext4`. This protects user data across VM lifecycles.

### 6. Compose file transfer: inline via heredoc, not SCP

Rather than implementing SCP/SFTP, the provisioner will read the compose file locally and write it to the VM using a shell heredoc in the SSH session:

```bash
cat > /data/docker-compose.yml << 'SERVERKU_EOF'
<compose content>
SERVERKU_EOF
```

This avoids adding SFTP complexity. The compose file is typically small (< 10KB).

**Alternative considered**: `sftp` subsystem via `golang.org/x/crypto/ssh`. Rejected because it adds complexity for a file that's always small. Can be added later if users need to transfer large files.

### 7. Orchestrator integration: inject via constructor, not ProviderFactory pattern

The provisioner will be passed to the `Orchestrator` constructor (like the notifier is today), not via a factory on each call. There's only one provisioner implementation and it doesn't vary per project.

```go
func New(store *config.Store, provisioner provisioner.Provisioner, notifier notify.Notifier) *Orchestrator
```

The CLI (`cmd/serverku/lifecycle.go`) will construct the provisioner and pass it in.

## Risks / Trade-offs

**[SSH connection timeout on slow VMs]** GCP SPOT instances can take 30-60 seconds before sshd starts. → Mitigation: Retry with exponential backoff up to 5 minutes. Log each retry attempt so the user sees progress.

**[Docker install script failure]** `get.docker.com` script could fail due to network issues or package repo problems. → Mitigation: Treat as a fatal provisioning error. The orchestrator already handles this: on provisioning failure, destroy VM but keep disk (per `docs/02-implementation-plan.md` Phase 5 error handling). User can retry with `serverku up`.

**[Disk already mounted at different path]** If a user changes `mount_path` between runs, the disk may have data at the old path. → Mitigation: The provisioner mounts wherever config says. The old path's data is still on the disk, just at a different relative location. Document this behavior.

**[Compose file not found]** If `compose_file` in config points to a nonexistent local file. → Mitigation: Validate file existence in `Provision()` before SSH-ing in. Return clear error.

**[Non-root user needs sudo]** The `serverku` SSH user is not root. All privileged commands (Docker install, disk mount, fstab) need `sudo`. → Mitigation: GCP auto-grants sudo to metadata-injected users. All scripts will use `sudo` for privileged operations.
