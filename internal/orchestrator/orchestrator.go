package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/hooks"
	"github.com/jufianto/serverku/internal/notify"
	"github.com/jufianto/serverku/internal/pricing"
	"github.com/jufianto/serverku/internal/provider"
	"github.com/jufianto/serverku/internal/provisioner"
)

// HookRunner runs a project's local lifecycle hook commands. It is an interface
// so tests can assert hook invocation without spawning shells.
type HookRunner interface {
	Run(ctx context.Context, phase string, commands []string, workDir string, env []string) error
}

// defaultHookRunner executes hooks via the hooks package, streaming output to
// the process stdout/stderr.
type defaultHookRunner struct{}

func (defaultHookRunner) Run(ctx context.Context, phase string, commands []string, workDir string, env []string) error {
	return hooks.Run(ctx, phase, commands, hooks.Options{WorkDir: workDir, Env: env})
}

// Orchestrator coordinates the lifecycle of serverku projects.
// It ties together the cloud provider, config store, and notifier to implement
// the core up/down/status/destroy operations.
type Orchestrator struct {
	store       *config.Store
	provisioner provisioner.Provisioner
	notifier    notify.Notifier
	hooks       HookRunner
}

// New creates a new Orchestrator.
// If prov is nil, a NoopProvisioner is used (no SSH provisioning).
// If notifier is nil, a NoopNotifier is used.
// If hookRunner is nil, a default runner that executes hooks via `sh -c` is used.
func New(store *config.Store, prov provisioner.Provisioner, notifier notify.Notifier, hookRunner HookRunner) *Orchestrator {
	if prov == nil {
		prov = &provisioner.NoopProvisioner{}
	}
	if notifier == nil {
		notifier = &notify.NoopNotifier{}
	}
	if hookRunner == nil {
		hookRunner = defaultHookRunner{}
	}
	return &Orchestrator{
		store:       store,
		provisioner: prov,
		notifier:    notifier,
		hooks:       hookRunner,
	}
}

// hookWorkDir returns the directory hooks run in: the project's sync_dir when
// set (where a local build naturally happens), else the current process dir.
func hookWorkDir(cfg *config.ProjectConfig) string {
	if cfg.SyncDir != "" {
		return cfg.SyncDir
	}
	return ""
}

// hookEnv builds the environment exposed to hook commands. ip may be empty when
// not yet known.
func hookEnv(cfg *config.ProjectConfig, ip string) []string {
	env := []string{
		"SERVERKU_PROJECT=" + cfg.Name,
		"SERVERKU_PROVIDER=" + cfg.Provider,
	}
	if ip != "" {
		env = append(env, "SERVERKU_IP="+ip)
	}
	return env
}

// ProviderFactory is a function that creates a CloudProvider for the given project config.
// This allows the orchestrator to remain decoupled from specific provider implementations.
type ProviderFactory func(ctx context.Context, cfg *config.ProjectConfig) (provider.CloudProvider, error)

// UpResult contains the result of a successful Up operation.
type UpResult struct {
	VMName     string
	VMID       string
	ExternalIP string
	DiskName   string
	DiskID     string
}

// Up creates a VM, optionally creates/attaches storage, and prepares the project for use.
// It does NOT provision the VM (install Docker, deploy containers) -- that's the provisioner's job.
//
// The operation flow:
//  1. Load config and state
//  2. Validate project is not already running
//  3. Ensure SSH keys exist
//  4. Create disk if storage enabled and disk doesn't exist yet
//  5. Create VM with SSH key injected
//  6. Attach disk if storage enabled
//  7. Wait for VM to be ready
//  8. Get external IP
//  9. Save state at each step for recovery
//  10. Send notification
func (o *Orchestrator) Up(ctx context.Context, projectName string, factory ProviderFactory) (*UpResult, error) {
	// Step 1: Load config and state
	cfg, err := o.store.LoadProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to load project: %w", err)
	}

	state, err := o.store.LoadState(projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Step 2: Validate not already running
	if state.IsRunning() {
		return nil, fmt.Errorf("project %q is already running (status: %s, ip: %s)", projectName, state.Status, state.ExternalIP)
	}

	// Step 2.5: pre_up hook -- runs locally before any cloud resource is created,
	// so e.g. a build can produce artifacts that are then synced to the VM.
	if err := o.hooks.Run(ctx, "pre_up", cfg.Hooks.PreUp, hookWorkDir(cfg), hookEnv(cfg, "")); err != nil {
		return nil, fmt.Errorf("pre_up hook failed: %w", err)
	}

	// Step 3: Ensure SSH keys
	privKeyPath, pubKey, err := o.store.EnsureSSHKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to ensure SSH keys: %w", err)
	}

	// Create the cloud provider
	cp, err := factory(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud provider: %w", err)
	}

	// Update state to starting
	now := time.Now()
	state.ProjectName = projectName
	state.Status = config.StatusStarting
	state.Provider = cfg.Provider
	state.Region = cfg.Region
	state.Zone = cfg.Zone
	state.StartedAt = &now
	state.ErrorMsg = ""
	if err := o.store.SaveState(state); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	result := &UpResult{}

	// Step 4: Create disk if storage is enabled and disk doesn't exist
	if cfg.Storage.Enabled && state.DiskName == "" {
		log.Printf("[orchestrator] creating persistent disk for project %q", projectName)

		diskName := fmt.Sprintf("serverku-%s-data", projectName)
		disk, err := cp.CreateDisk(ctx, provider.DiskConfig{
			Name:      diskName,
			Zone:      cfg.Zone,
			SizeGB:    int64(cfg.Storage.SizeGB),
			DiskType:  "pd-standard",
			ProjectID: cfg.ProjectID,
		})
		if err != nil {
			o.setErrorState(state, fmt.Sprintf("failed to create disk: %v", err))
			return nil, fmt.Errorf("failed to create disk: %w", err)
		}

		state.DiskID = disk.ID
		state.DiskName = disk.Name
		result.DiskID = disk.ID
		result.DiskName = disk.Name
		if err := o.store.SaveState(state); err != nil {
			return nil, fmt.Errorf("failed to save state after disk creation: %w", err)
		}

		log.Printf("[orchestrator] disk %q created (id: %s)", disk.Name, disk.ID)
	} else if cfg.Storage.Enabled {
		// Disk already exists from a previous run
		result.DiskID = state.DiskID
		result.DiskName = state.DiskName
		log.Printf("[orchestrator] reusing existing disk %q", state.DiskName)
	}

	// Step 4.5: Ensure firewall rules exist for providers that need them (e.g.
	// GCP's default network blocks inbound traffic). Done before the VM exists
	// so there is nothing to clean up on failure; the rule targets the
	// project's network tags, so it survives down/up cycles.
	if fw, ok := cp.(provider.FirewallManager); ok {
		log.Printf("[orchestrator] ensuring firewall rules for project %q", projectName)
		if err := fw.EnsureFirewall(ctx, projectName); err != nil {
			o.setErrorState(state, fmt.Sprintf("failed to ensure firewall: %v", err))
			return nil, fmt.Errorf("failed to ensure firewall: %w", err)
		}
	}

	// Step 5: Create VM
	vmName := fmt.Sprintf("serverku-%s", projectName)
	log.Printf("[orchestrator] creating VM %q", vmName)

	vm, err := cp.CreateVM(ctx, provider.VMConfig{
		Name:        vmName,
		Region:      cfg.Region,
		Zone:        cfg.Zone,
		MachineType: cfg.VM.Size,
		Image:       cfg.VM.Image,
		Spot:        cfg.VM.Spot,
		Tags:        []string{"serverku", fmt.Sprintf("serverku-%s", projectName)},
		SSHPubKey:   pubKey,
		ProjectID:   cfg.ProjectID,
	})
	if err != nil {
		// VM creation failed -- keep disk if it was created, don't clean up
		o.setErrorState(state, fmt.Sprintf("failed to create VM: %v", err))
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	state.VMID = vm.ID
	state.VMName = vm.Name
	result.VMID = vm.ID
	result.VMName = vm.Name
	if err := o.store.SaveState(state); err != nil {
		return nil, fmt.Errorf("failed to save state after VM creation: %w", err)
	}

	log.Printf("[orchestrator] VM %q created (id: %s)", vm.Name, vm.ID)

	// Step 6: Attach disk if storage is enabled
	if cfg.Storage.Enabled && state.DiskName != "" {
		log.Printf("[orchestrator] attaching disk %q to VM %q", state.DiskName, vm.Name)

		if err := cp.AttachDisk(ctx, vm.Name, state.DiskName); err != nil {
			// Disk attach failed -- destroy VM but keep disk
			log.Printf("[orchestrator] disk attach failed, destroying VM: %v", err)
			_ = cp.DestroyVM(ctx, vm.Name)
			o.setErrorState(state, fmt.Sprintf("failed to attach disk: %v", err))
			return nil, fmt.Errorf("failed to attach disk: %w", err)
		}
	}

	// Step 7: Wait for VM to be ready
	log.Printf("[orchestrator] waiting for VM %q to be ready", vm.Name)
	if err := cp.WaitForReady(ctx, vm.Name); err != nil {
		o.setErrorState(state, fmt.Sprintf("VM failed to become ready: %v", err))
		return nil, fmt.Errorf("VM failed to become ready: %w", err)
	}

	// Step 8: Get external IP
	ip, err := cp.GetExternalIP(ctx, vm.Name)
	if err != nil {
		o.setErrorState(state, fmt.Sprintf("failed to get external IP: %v", err))
		return nil, fmt.Errorf("failed to get external IP: %w", err)
	}

	result.ExternalIP = ip
	state.ExternalIP = ip
	// Save IP to state before provisioning so it's persisted if we crash
	if err := o.store.SaveState(state); err != nil {
		return nil, fmt.Errorf("failed to save state after getting IP: %w", err)
	}

	// Step 8.5: DNS automation. Done before provisioning so Caddyku/Let's Encrypt
	// can resolve the domains when acquiring certificates.
	if cfg.DNS.Enabled {
		dnsMgr, ok := cp.(provider.DNSManager)
		if !ok {
			o.setErrorState(state, fmt.Sprintf("dns automation is not yet supported for the %q provider", cfg.Provider))
			log.Printf("[orchestrator] DNS enabled but unsupported for provider %q, destroying VM (disk preserved)", cfg.Provider)
			_ = cp.DestroyVM(ctx, vm.Name)
			return nil, fmt.Errorf("dns automation is not yet supported for the %q provider", cfg.Provider)
		}
		for _, d := range cfg.Router.Domains {
			log.Printf("[orchestrator] ensuring DNS A record %s -> %s", d.Domain, ip)
			if err := dnsMgr.EnsureARecord(ctx, d.Domain, ip, cfg.DNS.TTL); err != nil {
				o.setErrorState(state, fmt.Sprintf("failed to set DNS record for %s: %v", d.Domain, err))
				log.Printf("[orchestrator] DNS update failed, destroying VM (disk preserved): %v", err)
				_ = cp.DestroyVM(ctx, vm.Name)
				return nil, fmt.Errorf("failed to set DNS record for %s: %w", d.Domain, err)
			}
		}
	}

	// Step 9: Provision the VM (install Docker, mount disk, deploy compose)
	log.Printf("[orchestrator] provisioning VM %q at %s...", vm.Name, ip)

	// Read the compose file content locally before SSH-ing in.
	var composeContent string
	if cfg.ComposeFile != "" {
		data, err := os.ReadFile(cfg.ComposeFile)
		if err != nil {
			o.setErrorState(state, fmt.Sprintf("compose file not found: %v", err))
			// Destroy VM on provisioning pre-check failure, keep disk.
			log.Printf("[orchestrator] compose file not found, destroying VM")
			_ = cp.DestroyVM(ctx, vm.Name)
			return nil, fmt.Errorf("compose file %q not found: %w", cfg.ComposeFile, err)
		}
		composeContent = string(data)
	}

	provOpts := provisioner.ProvisionOpts{
		Host:            ip,
		PrivateKeyPath:  privKeyPath,
		SSHUser:         "serverku",
		StorageEnabled:  cfg.Storage.Enabled,
		DiskName:        state.DiskName,
		MountPath:       cfg.Storage.MountPath,
		ComposeContent:  composeContent,
		SyncDir:         cfg.SyncDir,
		RouterEnabled:   cfg.Router.Enabled,
		Domains:         cfg.Router.Domains,
		StartupCommands: cfg.StartupCommands,
		Heartbeat:       heartbeatOpts(ctx, cfg, cp, projectName),
	}
	if err := o.provisioner.Provision(ctx, provOpts); err != nil {
		o.setErrorState(state, fmt.Sprintf("provisioning failed: %v", err))
		// Destroy VM on provisioning failure; keep disk (data must survive).
		log.Printf("[orchestrator] provisioning failed, destroying VM (disk preserved): %v", err)
		_ = cp.DestroyVM(ctx, vm.Name)
		return nil, fmt.Errorf("failed to provision VM: %w", err)
	}

	state.Status = config.StatusRunning
	if err := o.store.SaveState(state); err != nil {
		return nil, fmt.Errorf("failed to save final state: %w", err)
	}

	log.Printf("[orchestrator] project %q is running at %s", projectName, ip)

	// Step 10: Send notification
	if err := o.notifier.SendUp(ctx, projectName, ip); err != nil {
		log.Printf("[orchestrator] failed to send notification: %v", err)
		// Non-fatal: don't fail the operation because of notifications
	}

	// Step 11: post_up hook -- runs after the project is up. Best-effort: a
	// failure is logged but does not fail the operation (the VM is already up).
	if err := o.hooks.Run(ctx, "post_up", cfg.Hooks.PostUp, hookWorkDir(cfg), hookEnv(cfg, ip)); err != nil {
		log.Printf("[orchestrator] post_up hook failed (non-fatal): %v", err)
	}

	return result, nil
}

// Down tears down the VM for a project but preserves persistent storage.
//
// The operation flow:
//  1. Load config and state
//  2. Validate project is running
//  3. Detach disk if storage is enabled
//  4. Destroy VM
//  5. Update state (keep disk info, clear VM info)
//  6. Send notification
func (o *Orchestrator) Down(ctx context.Context, projectName string, factory ProviderFactory) error {
	return o.down(ctx, projectName, factory, false)
}

// down is the implementation of Down. When suppressHooks is true the pre_down /
// post_down hooks are skipped -- used when Destroy calls down internally so that
// a `destroy` fires only its own hooks, not the `down` hooks.
func (o *Orchestrator) down(ctx context.Context, projectName string, factory ProviderFactory, suppressHooks bool) error {
	// Step 1: Load config and state
	cfg, err := o.store.LoadProject(projectName)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	state, err := o.store.LoadState(projectName)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Step 2: Validate project is running
	if !state.IsRunning() && state.VMName == "" {
		return fmt.Errorf("project %q is not running (status: %s)", projectName, state.Status)
	}

	// Step 2.1: pre_down hook -- runs locally before teardown (e.g. back up data).
	if !suppressHooks {
		if err := o.hooks.Run(ctx, "pre_down", cfg.Hooks.PreDown, hookWorkDir(cfg), hookEnv(cfg, state.ExternalIP)); err != nil {
			return fmt.Errorf("pre_down hook failed: %w", err)
		}
	}

	// Create the cloud provider
	cp, err := factory(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create cloud provider: %w", err)
	}

	// Update state to stopping
	state.Status = config.StatusStopping
	if err := o.store.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Step 2.5: Teardown the VM (stop containers, unmount disk) before detaching
	if state.ExternalIP != "" {
		privKeyPath, err := o.store.GetSSHPrivateKeyPath()
		if err != nil {
			log.Printf("[orchestrator] warning: could not get SSH key for teardown: %v", err)
		} else {
			composeDir := ""
			if cfg.Storage.Enabled && cfg.Storage.MountPath != "" {
				composeDir = cfg.Storage.MountPath
			} else if cfg.ComposeFile != "" {
				composeDir = "/home/serverku"
			}
			teardownOpts := provisioner.TeardownOpts{
				Host:           state.ExternalIP,
				PrivateKeyPath: privKeyPath,
				SSHUser:        "serverku",
				StorageEnabled: cfg.Storage.Enabled,
				MountPath:      cfg.Storage.MountPath,
				ComposeDir:     composeDir,
			}
			if err := o.provisioner.Teardown(ctx, teardownOpts); err != nil {
				// Non-fatal: log and continue
				log.Printf("[orchestrator] warning: teardown error (continuing): %v", err)
			}
		}
	}

	// Step 3: Detach disk if storage is enabled
	if cfg.Storage.Enabled && state.DiskName != "" && state.VMName != "" {
		log.Printf("[orchestrator] detaching disk %q from VM %q", state.DiskName, state.VMName)

		if err := cp.DetachDisk(ctx, state.VMName, state.DiskName); err != nil {
			log.Printf("[orchestrator] warning: failed to detach disk (may already be detached): %v", err)
			// Continue with VM destruction even if detach fails
		}
	}

	// Step 4: Destroy VM
	if state.VMName != "" {
		log.Printf("[orchestrator] destroying VM %q", state.VMName)

		if err := cp.DestroyVM(ctx, state.VMName); err != nil {
			o.setErrorState(state, fmt.Sprintf("failed to destroy VM: %v", err))
			return fmt.Errorf("failed to destroy VM: %w", err)
		}
	}

	// Step 5: Update state
	now := time.Now()
	state.Status = config.StatusStopped
	state.VMID = ""
	state.VMName = ""
	state.ExternalIP = ""
	state.StoppedAt = &now
	state.ErrorMsg = ""
	// Keep DiskID and DiskName -- disk persists across VM lifecycles
	if err := o.store.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	log.Printf("[orchestrator] project %q is stopped", projectName)

	// Step 6: Send notification
	if err := o.notifier.SendDown(ctx, projectName); err != nil {
		log.Printf("[orchestrator] failed to send notification: %v", err)
	}

	// Step 7: post_down hook -- runs after teardown. Best-effort (VM is gone).
	if !suppressHooks {
		if err := o.hooks.Run(ctx, "post_down", cfg.Hooks.PostDown, hookWorkDir(cfg), hookEnv(cfg, "")); err != nil {
			log.Printf("[orchestrator] post_down hook failed (non-fatal): %v", err)
		}
	}

	return nil
}

// Status checks the current status of a project, reconciling local state with
// the cloud provider if a VM ID is tracked.
func (o *Orchestrator) Status(ctx context.Context, projectName string, factory ProviderFactory) (*config.ProjectState, error) {
	cfg, err := o.store.LoadProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to load project: %w", err)
	}

	state, err := o.store.LoadState(projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// If we have a VM tracked, verify with the provider that it still exists
	if state.VMName != "" {
		cp, err := factory(ctx, cfg)
		if err != nil {
			// Can't reach provider -- return local state with a warning
			log.Printf("[orchestrator] warning: could not create provider to verify VM status: %v", err)
			return state, nil
		}

		vmStatus, err := cp.GetVMStatus(ctx, state.VMName)
		if err != nil {
			log.Printf("[orchestrator] warning: could not verify VM status: %v", err)
			return state, nil
		}

		// Reconcile: if the VM was terminated externally (e.g., SPOT reclaimed),
		// update our local state to match
		switch vmStatus.State {
		case provider.VMStateTerminated:
			log.Printf("[orchestrator] VM %q was terminated externally, updating state", state.VMName)
			now := time.Now()
			state.Status = config.StatusStopped
			state.VMID = ""
			state.VMName = ""
			state.ExternalIP = ""
			state.StoppedAt = &now
			_ = o.store.SaveState(state)

		case provider.VMStateRunning:
			state.Status = config.StatusRunning
			if vmStatus.ExternalIP != "" {
				state.ExternalIP = vmStatus.ExternalIP
			}
			_ = o.store.SaveState(state)

		case provider.VMStateStopped:
			state.Status = config.StatusStopped
			state.ExternalIP = ""
			_ = o.store.SaveState(state)

		case provider.VMStateStarting:
			state.Status = config.StatusStarting
			_ = o.store.SaveState(state)

		case provider.VMStateStopping:
			state.Status = config.StatusStopping
			_ = o.store.SaveState(state)
		}
	}

	return state, nil
}

// Destroy permanently deletes all cloud resources and local config for a project.
// This includes the VM, persistent disk, firewall rules, state file, and config file.
//
// The operation flow:
//  1. Down() if running (tear down VM, detach disk)
//  2. Delete disk if exists
//  3. Delete local state and config files
func (o *Orchestrator) Destroy(ctx context.Context, projectName string, factory ProviderFactory) error {
	cfg, err := o.store.LoadProject(projectName)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	state, err := o.store.LoadState(projectName)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Step 0: pre_destroy hook -- runs locally before anything is deleted.
	if err := o.hooks.Run(ctx, "pre_destroy", cfg.Hooks.PreDestroy, hookWorkDir(cfg), hookEnv(cfg, state.ExternalIP)); err != nil {
		return fmt.Errorf("pre_destroy hook failed: %w", err)
	}

	// Step 1: If running, bring it down first. Hooks are suppressed here so a
	// destroy fires only pre_destroy/post_destroy, not the down hooks.
	if state.IsRunning() || state.VMName != "" {
		log.Printf("[orchestrator] project %q has a VM, tearing down first", projectName)
		if err := o.down(ctx, projectName, factory, true); err != nil {
			return fmt.Errorf("failed to tear down before destroy: %w", err)
		}
		// Reload state after Down (it will have cleared VM info)
		state, err = o.store.LoadState(projectName)
		if err != nil {
			return fmt.Errorf("failed to reload state: %w", err)
		}
	}

	// Step 2: Delete cloud-side leftovers (disk, firewall rules). The provider
	// is needed for both; failing to construct it is only fatal when a disk
	// still has to be deleted.
	cp, cpErr := factory(ctx, cfg)
	if cpErr != nil {
		if state.DiskName != "" {
			return fmt.Errorf("failed to create cloud provider: %w", cpErr)
		}
		log.Printf("[orchestrator] warning: could not create provider for firewall cleanup: %v", cpErr)
	} else {
		if state.DiskName != "" {
			log.Printf("[orchestrator] deleting disk %q", state.DiskName)
			if err := cp.DeleteDisk(ctx, state.DiskName); err != nil {
				return fmt.Errorf("failed to delete disk: %w", err)
			}
		}

		// Firewall cleanup is best-effort: the rule is harmless on its own and
		// the project is going away either way.
		if fw, ok := cp.(provider.FirewallManager); ok {
			log.Printf("[orchestrator] deleting firewall rules for project %q", projectName)
			if err := fw.DeleteFirewall(ctx, projectName); err != nil {
				log.Printf("[orchestrator] warning: failed to delete firewall rules (continuing): %v", err)
			}
		}
	}

	// Step 3: Delete local state and config
	if err := o.store.DeleteState(projectName); err != nil {
		return fmt.Errorf("failed to delete state: %w", err)
	}
	if err := o.store.DeleteProject(projectName); err != nil {
		return fmt.Errorf("failed to delete project config: %w", err)
	}

	log.Printf("[orchestrator] project %q destroyed", projectName)

	// Step 4: post_destroy hook -- best-effort local cleanup after destroy.
	if err := o.hooks.Run(ctx, "post_destroy", cfg.Hooks.PostDestroy, hookWorkDir(cfg), hookEnv(cfg, "")); err != nil {
		log.Printf("[orchestrator] post_destroy hook failed (non-fatal): %v", err)
	}

	return nil
}

// BackupResult contains the result of a successful Backup operation.
type BackupResult struct {
	SnapshotName string
	SnapshotID   string
	DiskName     string
}

// Backup creates a snapshot of the project's persistent disk. The disk does not
// need to be attached; the project may be up or down. snapshotName is supplied
// by the caller (the CLI stamps it with a timestamp) so the orchestrator stays
// deterministic and testable.
func (o *Orchestrator) Backup(ctx context.Context, projectName, snapshotName string, factory ProviderFactory) (*BackupResult, error) {
	cfg, err := o.store.LoadProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to load project: %w", err)
	}

	state, err := o.store.LoadState(projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	if !cfg.Storage.Enabled {
		return nil, fmt.Errorf("project %q has no persistent storage to back up", projectName)
	}
	if state.DiskName == "" {
		return nil, fmt.Errorf("project %q has no disk yet; run `serverku up` first", projectName)
	}

	cp, err := factory(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud provider: %w", err)
	}

	log.Printf("[orchestrator] snapshotting disk %q as %q", state.DiskName, snapshotName)
	snapshotID, err := cp.SnapshotDisk(ctx, state.DiskName, snapshotName)
	if err != nil {
		return nil, fmt.Errorf("failed to snapshot disk: %w", err)
	}

	return &BackupResult{
		SnapshotName: snapshotName,
		SnapshotID:   snapshotID,
		DiskName:     state.DiskName,
	}, nil
}

// heartbeatOpts assembles the on-VM Telegram heartbeat settings from config.
// The accrued-cost rate prefers the provider's live pricing API (PriceCatalog
// capability) and falls back to the offline table, preserving provenance so
// the heartbeat message marks estimates as est.
func heartbeatOpts(ctx context.Context, cfg *config.ProjectConfig, cp provider.CloudProvider, projectName string) provisioner.HeartbeatOpts {
	hb := provisioner.HeartbeatOpts{ProjectName: projectName}

	// Telegram channel: account-linked bot token, root-only on the VM.
	if tg := cfg.Notifications.Telegram; tg.HeartbeatHours > 0 && tg.BotToken != "" && tg.ChatID != "" {
		hb.Hours = tg.HeartbeatHours
		hb.BotToken = tg.BotToken
		hb.ChatID = tg.ChatID
	}

	// ntfy channel: account-less, the topic name is the only capability.
	// When both channels are configured, the smaller interval wins.
	if nt := cfg.Notifications.Ntfy; nt.HeartbeatHours > 0 && nt.Topic != "" {
		if hb.Hours == 0 || nt.HeartbeatHours < hb.Hours {
			hb.Hours = nt.HeartbeatHours
		}
		hb.NtfyServer = nt.ServerURL()
		hb.NtfyTopic = nt.Topic
	}

	if hb.Hours == 0 {
		return provisioner.HeartbeatOpts{}
	}

	rate := pricing.TableRate(cfg)
	if pc, ok := cp.(provider.PriceCatalog); ok {
		if hourly, err := pc.VMHourlyRateUSD(ctx, cfg.VM.Size, cfg.Region, cfg.VM.Spot); err == nil {
			rate = pricing.Rate{HourlyUSD: hourly, Live: true, Known: true}
		} else {
			log.Printf("[orchestrator] live rate lookup for heartbeat failed, using offline estimate: %v", err)
		}
	}
	if rate.Known {
		hb.HourlyRateUSD = rate.HourlyUSD
		hb.RateIsLive = rate.Live
	}

	return hb
}

// setErrorState updates the project state to error status and saves it.
func (o *Orchestrator) setErrorState(state *config.ProjectState, errMsg string) {
	state.Status = config.StatusError
	state.ErrorMsg = errMsg
	if err := o.store.SaveState(state); err != nil {
		log.Printf("[orchestrator] failed to save error state: %v", err)
	}

	// Send error notification
	if err := o.notifier.SendError(context.Background(), state.ProjectName, fmt.Errorf("%s", errMsg)); err != nil {
		log.Printf("[orchestrator] failed to send error notification: %v", err)
	}
}
