## ADDED Requirements

### Requirement: Directory Synchronization
The project configuration SHALL support a `SyncDir` string field. If provided, `serverku` SHALL synchronize the contents of this local directory to the remote VM before deploying containers.

#### Scenario: SyncDir is configured
- **WHEN** `SyncDir` points to a valid local directory
- **THEN** the `SSHProvisioner` SHALL use `rsync` over SSH to copy the directory contents to the target deployment directory on the VM.

#### Scenario: Common exclusions are applied
- **WHEN** performing the directory synchronization
- **THEN** the `rsync` command SHALL exclude common large/unnecessary directories like `.git`, `node_modules`, and `vendor`.

#### Scenario: rsync binary is missing
- **WHEN** `SyncDir` is configured but the `rsync` binary is not found in the local system `$PATH`
- **THEN** `serverku` SHALL return an error instructing the user to install `rsync`.
