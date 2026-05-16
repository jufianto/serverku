## 1. Configuration Setup

- [x] 1.1 Add `SyncDir` (string) field to `ProjectConfig` in `internal/config/project.go`.
- [x] 1.2 Add `SyncDir` field to `ProvisionOpts` in `internal/provisioner/provisioner.go`.
- [x] 1.3 Update `cmd/serverku/lifecycle.go` to pass `cfg.SyncDir` into `ProvisionOpts`.

## 2. Sync Implementation

- [x] 2.1 In `internal/provisioner/provisioner.go`, update `Provision()` to check if `opts.SyncDir` is set.
- [x] 2.2 Verify `exec.LookPath("rsync")` exists, returning an error if not.
- [x] 2.3 Implement the rsync command using `exec.Command` with args: `-avz`, `-e "ssh -i <privkey> -o StrictHostKeyChecking=accept-new"`, `--exclude=.git`, `--exclude=node_modules`, `--exclude=vendor`, `<opts.SyncDir>/`, `<opts.SSHUser>@<opts.Host>:<composeDir>/`.
- [x] 2.4 Execute the rsync command before the docker-compose deployment step.