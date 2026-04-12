## ADDED Requirements

### Requirement: Provisioner interface
The system SHALL define a `Provisioner` interface in `internal/provisioner/provisioner.go` with two methods: `Provision(ctx, ProvisionOpts) error` and `Teardown(ctx, TeardownOpts) error`. All orchestrator interactions with provisioning SHALL go through this interface.

#### Scenario: Orchestrator uses provisioner interface
- **WHEN** the orchestrator calls `Provision()` or `Teardown()`
- **THEN** it SHALL use the `Provisioner` interface, never a concrete implementation directly

### Requirement: SSH connection with retry
The SSH provisioner SHALL connect to the VM using the serverku-generated RSA private key at `~/.serverku/keys/serverku_rsa` as user `serverku`. It SHALL retry the connection with exponential backoff (starting at 5 seconds, doubling each attempt) for up to 5 minutes total before returning an error.

#### Scenario: VM SSH not immediately available
- **WHEN** the provisioner attempts to connect and the SSH server is not yet ready
- **THEN** it SHALL retry with exponential backoff (5s, 10s, 20s, 40s...) up to 5 minutes total

#### Scenario: VM SSH becomes available
- **WHEN** the SSH connection succeeds within the retry window
- **THEN** provisioning SHALL proceed with Docker installation

#### Scenario: VM SSH never becomes available
- **WHEN** 5 minutes of retries elapse without a successful connection
- **THEN** the provisioner SHALL return an error indicating SSH timeout

### Requirement: Docker installation
The provisioner SHALL install Docker Engine and Docker Compose plugin on the VM by executing the official `get.docker.com` install script. It SHALL add the `serverku` user to the `docker` group and enable the Docker systemd service.

#### Scenario: Fresh VM with no Docker
- **WHEN** provisioning runs on a VM without Docker installed
- **THEN** Docker Engine and Docker Compose plugin SHALL be installed and the Docker service SHALL be running

#### Scenario: Docker install script fails
- **WHEN** the Docker installation script returns a non-zero exit code
- **THEN** the provisioner SHALL return an error and provisioning SHALL stop

### Requirement: Disk formatting for new disks
When storage is enabled, the provisioner SHALL check whether the attached disk has an existing filesystem using `blkid`. If no filesystem is detected, it SHALL format the disk with `mkfs.ext4`. It SHALL NEVER format a disk that already has a filesystem.

#### Scenario: First-time disk with no filesystem
- **WHEN** `blkid` returns non-zero for the disk device
- **THEN** the provisioner SHALL format the disk with `mkfs.ext4`

#### Scenario: Returning disk with existing filesystem
- **WHEN** `blkid` returns zero (filesystem detected) for the disk device
- **THEN** the provisioner SHALL NOT format the disk

### Requirement: Disk mounting
When storage is enabled, the provisioner SHALL mount the persistent disk to the path specified in `ProjectConfig.Storage.MountPath`. The disk device path SHALL be `/dev/disk/by-id/google-<disk-name>` (not `/dev/sdb`). The provisioner SHALL add an entry to `/etc/fstab` for persistence across reboots (using `nofail` option).

#### Scenario: Mount disk to configured path
- **WHEN** storage is enabled and the disk is attached
- **THEN** the provisioner SHALL mount the disk at the configured `mount_path` and add an fstab entry

#### Scenario: Storage not enabled
- **WHEN** storage is not enabled in the project config
- **THEN** the provisioner SHALL skip all disk formatting and mounting steps

### Requirement: Compose file transfer and deployment
When `ProjectConfig.ComposeFile` is set, the provisioner SHALL read the file from the local filesystem, transfer its contents to the VM via SSH heredoc, and place it at `<mount_path>/docker-compose.yml` (if storage is enabled) or `/home/serverku/docker-compose.yml` (if storage is disabled). It SHALL then run `docker compose up -d` in the directory containing the compose file.

#### Scenario: Compose file exists locally
- **WHEN** `compose_file` is set and the file exists on the local machine
- **THEN** the provisioner SHALL transfer it to the VM and run `docker compose up -d`

#### Scenario: Compose file does not exist locally
- **WHEN** `compose_file` is set but the file does not exist on the local machine
- **THEN** the provisioner SHALL return an error before attempting SSH

#### Scenario: Compose file not configured
- **WHEN** `compose_file` is empty/unset in the project config
- **THEN** the provisioner SHALL skip compose file transfer and `docker compose up`

### Requirement: Startup commands execution
When `ProjectConfig.StartupCommands` is non-empty, the provisioner SHALL execute each command in order on the VM via SSH after Docker installation and compose deployment are complete. If any command returns a non-zero exit code, the provisioner SHALL return an error.

#### Scenario: Startup commands succeed
- **WHEN** all startup commands execute with exit code 0
- **THEN** provisioning SHALL complete successfully

#### Scenario: A startup command fails
- **WHEN** a startup command returns a non-zero exit code
- **THEN** the provisioner SHALL return an error with the command and its output

#### Scenario: No startup commands configured
- **WHEN** `startup_commands` is empty or unset
- **THEN** the provisioner SHALL skip this step

### Requirement: Teardown stops containers and unmounts disk
The `Teardown` method SHALL connect to the VM via SSH, run `docker compose down` in the compose file directory (if compose was deployed), and unmount the persistent disk (if storage is enabled) using `sudo umount <mount_path>`.

#### Scenario: Full teardown with compose and storage
- **WHEN** teardown is called for a project with compose and storage
- **THEN** it SHALL run `docker compose down`, then `sudo umount <mount_path>`

#### Scenario: Teardown with no compose file
- **WHEN** teardown is called and no compose file was configured
- **THEN** it SHALL skip `docker compose down` and only unmount if storage is enabled

#### Scenario: Teardown SSH connection fails
- **WHEN** teardown cannot connect to the VM via SSH (e.g., VM already stopping)
- **THEN** it SHALL log a warning and return without error (non-fatal, VM is being destroyed anyway)

### Requirement: Orchestrator integration
The `Orchestrator.Up()` method SHALL call `provisioner.Provision()` after obtaining the external IP and before marking the project as `running`. The `Orchestrator.Down()` method SHALL call `provisioner.Teardown()` before detaching the disk. If provisioning fails, `Up()` SHALL destroy the VM but preserve the disk.

#### Scenario: Provisioning succeeds in Up flow
- **WHEN** `Provision()` returns nil
- **THEN** the orchestrator SHALL mark the project as `running` and save the state

#### Scenario: Provisioning fails in Up flow
- **WHEN** `Provision()` returns an error
- **THEN** the orchestrator SHALL destroy the VM, preserve the disk, set error state, and return the error

#### Scenario: Teardown called in Down flow
- **WHEN** `Down()` is called for a running project
- **THEN** the orchestrator SHALL call `Teardown()` before detaching the disk and destroying the VM
