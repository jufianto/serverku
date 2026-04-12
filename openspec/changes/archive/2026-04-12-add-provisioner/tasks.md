## 1. Provisioner Interface and Types

- [x] 1.1 Create `internal/provisioner/provisioner.go` with the `Provisioner` interface (`Provision(ctx, ProvisionOpts) error`, `Teardown(ctx, TeardownOpts) error`) and the `ProvisionOpts`/`TeardownOpts` structs. `ProvisionOpts` needs: `Host string`, `PrivateKeyPath string`, `SSHUser string`, `StorageEnabled bool`, `DiskName string`, `MountPath string`, `ComposeContent string` (already-read file content), `StartupCommands []string`. `TeardownOpts` needs: `Host string`, `PrivateKeyPath string`, `SSHUser string`, `StorageEnabled bool`, `MountPath string`, `ComposeDir string`.
- [x] 1.2 Create a `NoopProvisioner` in the same file (like `NoopNotifier` in `internal/notify/notify.go`) that returns nil from both methods. This is used when the orchestrator is constructed without a real provisioner (e.g., in existing orchestrator tests).

## 2. SSH Client Wrapper

- [x] 2.1 Create `internal/provisioner/ssh.go` with a function `connectSSH(ctx context.Context, host string, privateKeyPath string, user string) (*ssh.Client, error)` that: reads the private key file, parses it with `ssh.ParsePrivateKey`, creates an `ssh.ClientConfig` with `ssh.PublicKeys` auth and `ssh.InsecureIgnoreHostKey()` for host key callback (these are ephemeral VMs), and dials `host:22`. Use the context for cancellation.
- [x] 2.2 Add a function `connectSSHWithRetry(ctx context.Context, host string, privateKeyPath string, user string) (*ssh.Client, error)` that wraps `connectSSH` with exponential backoff: initial delay 5s, doubling each attempt, up to 5 minutes total. Log each retry attempt with `log.Printf`. Return a clear error on timeout: `"SSH connection timed out after 5 minutes"`.
- [x] 2.3 Add a function `runCommand(client *ssh.Client, command string) (string, error)` that opens a new session, runs `session.CombinedOutput(command)`, and returns the output as a string. On non-zero exit, return the output in the error message so the user can debug.

## 3. Shell Scripts

- [x] 3.1 Create `internal/provisioner/scripts.go` with a Go constant `installDockerScript` containing the shell script to install Docker: `sudo curl -fsSL https://get.docker.com | sudo sh && sudo usermod -aG docker serverku && sudo systemctl enable docker && sudo systemctl start docker`. Note: the user is `serverku` (hardcoded for now, matches GCP metadata SSH user).
- [x] 3.2 Add a function `mountDiskScript(diskName string, mountPath string) string` that returns a shell script to: (a) check with `sudo blkid /dev/disk/by-id/google-<diskName>`, (b) if blkid returns non-zero run `sudo mkfs.ext4 /dev/disk/by-id/google-<diskName>`, (c) `sudo mkdir -p <mountPath>`, (d) `sudo mount /dev/disk/by-id/google-<diskName> <mountPath>`, (e) add fstab entry with `nofail` option only if not already present (grep fstab first), (f) `sudo chown serverku:serverku <mountPath>`.
- [x] 3.3 Add a function `writeComposeScript(composeContent string, targetDir string) string` that returns a shell script to write the compose file via heredoc: `cat > <targetDir>/docker-compose.yml << 'SERVERKU_EOF'\n<content>\nSERVERKU_EOF`. Use a unique delimiter that won't appear in compose files.
- [x] 3.4 Add a function `composeUpScript(composeDir string) string` returning: `cd <composeDir> && docker compose up -d`.
- [x] 3.5 Add a function `teardownScript(composeDir string, mountPath string, hasCompose bool, hasStorage bool) string` returning a script that: (a) if hasCompose: `cd <composeDir> && docker compose down`, (b) if hasStorage: `sudo umount <mountPath>`. Each step should be best-effort (use `|| true` for umount since the VM is being destroyed).

## 4. SSH Provisioner Implementation

- [x] 4.1 Create the `SSHProvisioner` struct in `internal/provisioner/provisioner.go` (or a new file `internal/provisioner/ssh_provisioner.go` if the file is getting long). It needs no fields -- all state comes from `ProvisionOpts`/`TeardownOpts`.
- [x] 4.2 Implement `SSHProvisioner.Provision(ctx, opts)`: (a) If `opts.ComposeContent` is non-empty but empty string, skip compose steps. (b) Call `connectSSHWithRetry(ctx, opts.Host, opts.PrivateKeyPath, opts.SSHUser)`. (c) Run `installDockerScript` via `runCommand`. (d) If `opts.StorageEnabled`: run `mountDiskScript(opts.DiskName, opts.MountPath)`. (e) If `opts.ComposeContent != ""`: determine compose dir (`opts.MountPath` if storage enabled, else `/home/serverku`), run `writeComposeScript`, then `composeUpScript`. (f) For each `opts.StartupCommands`: run via `runCommand`, return error on failure. (g) Close SSH client.
- [x] 4.3 Implement `SSHProvisioner.Teardown(ctx, opts)`: (a) Attempt `connectSSH` (NOT `connectSSHWithRetry` -- if the VM is shutting down, don't wait 5 minutes). If connection fails, log a warning and return nil (teardown is best-effort). (b) Run `teardownScript`. (c) Close SSH client.

## 5. Orchestrator Integration

- [x] 5.1 Modify `Orchestrator` struct in `internal/orchestrator/orchestrator.go`: add `provisioner` field of type `provisioner.Provisioner`. Update `New()` to accept a `provisioner.Provisioner` parameter (between `store` and `notifier`). If nil is passed, use `&provisioner.NoopProvisioner{}`.
- [x] 5.2 Modify `Up()` in the orchestrator: after getting the external IP (after line 196 `result.ExternalIP = ip`) and before setting status to `StatusRunning` (line 197), add a provisioner call. Read the compose file content from the local filesystem using `cfg.ComposeFile` (if set, use `os.ReadFile`; if the file doesn't exist, set error state and return). Build `ProvisionOpts{Host: ip, PrivateKeyPath: privKeyPath, SSHUser: "serverku", StorageEnabled: cfg.Storage.Enabled, DiskName: state.DiskName, MountPath: cfg.Storage.MountPath, ComposeContent: <content>, StartupCommands: cfg.StartupCommands}`. Note: `privKeyPath` comes from `EnsureSSHKeys()` earlier in the function -- store the return value in step 3 (currently only `pubKey` is captured; also capture `privKeyPath`). If `Provision()` fails: destroy VM (`cp.DestroyVM`), set error state, return error. Keep disk intact.
- [x] 5.3 Modify `Down()` in the orchestrator: after creating the cloud provider and setting status to `StatusStopping`, before detaching the disk (before line 252), call `provisioner.Teardown()` with `TeardownOpts{Host: state.ExternalIP, PrivateKeyPath: <get from store>, SSHUser: "serverku", StorageEnabled: cfg.Storage.Enabled, MountPath: cfg.Storage.MountPath, ComposeDir: <mount_path or /home/serverku>}`. Get `PrivateKeyPath` by calling `o.store.GetSSHPrivateKeyPath()`. If teardown fails, log a warning but continue with Down (non-fatal).
- [x] 5.4 Update all callers of `orchestrator.New()` in `cmd/serverku/lifecycle.go` to pass a provisioner. Construct an `&provisioner.SSHProvisioner{}` and pass it to `orchestrator.New()`. Add the import for `internal/provisioner`.

## 6. Tests

- [x] 6.1 Create `internal/provisioner/provisioner_test.go`. Write unit tests for the script-generation functions: `mountDiskScript`, `writeComposeScript`, `composeUpScript`, `teardownScript`. Verify they produce correct shell with expected paths, disk names, heredoc delimiters, and `nofail` fstab option.
- [x] 6.2 Write unit tests for `NoopProvisioner` -- verify both methods return nil.
- [x] 6.3 Update `internal/orchestrator/orchestrator_test.go`: the `New()` constructor signature changed (added provisioner param). Update all `New()` calls to pass `&provisioner.NoopProvisioner{}` (or `nil` if the constructor handles it). Verify existing 12 tests still pass.
- [x] 6.4 Add orchestrator test: `TestUpCallsProvisioner` -- use a mock provisioner that records calls, verify `Provision()` was called with correct opts (host = external IP, correct SSH user, correct disk name, etc.) after a successful Up.
- [x] 6.5 Add orchestrator test: `TestUpProvisioningFailureDestroysVM` -- mock provisioner returns error, verify VM was destroyed, disk was preserved, state is `error`.
- [x] 6.6 Add orchestrator test: `TestDownCallsTeardown` -- verify `Teardown()` is called before disk detach and VM destroy.
- [x] 6.7 Run `go test ./...` and verify all tests pass (existing + new). Run `go build -o serverku ./cmd/serverku` and verify it compiles.

## 7. Bug Fixes (While We're Here)

- [x] 7.1 Fix `cmd/serverku/status.go`: currently reads state directly from store (lines 18-33) instead of calling `orch.Status()`. Change it to call `orch.Status(ctx, projectName, providerFactory)` so SPOT termination detection works. This requires the status command to have access to the provider factory (same pattern as up/down).
