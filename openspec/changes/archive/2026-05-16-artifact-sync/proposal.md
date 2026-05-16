## Why

Real-world Docker projects are rarely just a single `docker-compose.yml` file. They often include `.env` files for secrets, custom configuration files (like `nginx.conf`), or source code directories when using `build: .`. Currently, `serverku` only transfers the compose file, which breaks most real-world deployments. We need to synchronize the entire project directory to the remote VM.

## What Changes

- Update `ProjectConfig` to support a `SyncDir` field.
- Modify the `SSHProvisioner` to securely transfer the contents of `SyncDir` from the local machine to the VM's persistent storage before running `docker compose up`.
- We will wrap the system `scp` or `rsync` command using `os/exec` to perform the file transfer, leveraging the existing managed SSH private key.

## Capabilities

### New Capabilities
- `artifact-sync`: Securely transfer local project directories to the remote VM during provisioning.

### Modified Capabilities
- `<none>`

## Impact

- **Code:** Modifies `internal/config/project.go` and `internal/provisioner/provisioner.go`.
- **Dependencies:** Relies on the host system having `scp` or `rsync` installed in the `$PATH`.
- **Systems:** Allows full projects to run seamlessly on the remote VM.
