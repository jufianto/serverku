package provisioner

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/jufianto/serverku/internal/config"
)

// Provisioner defines the interface for setting up a VM after it has been
// created and is accessible via SSH.
type Provisioner interface {
	// Provision installs Docker, mounts storage, and deploys the compose stack
	// on a freshly created VM.
	Provision(ctx context.Context, opts ProvisionOpts) error

	// Deploy pushes the current project to an already-provisioned, running
	// VM: re-syncs the project directory, rewrites the compose file, and
	// runs `docker compose up -d`. Docker, storage, and routing from the
	// original Provision are reused untouched.
	Deploy(ctx context.Context, opts DeployOpts) error

	// Teardown stops running containers and unmounts storage before the VM
	// is destroyed.
	Teardown(ctx context.Context, opts TeardownOpts) error
}

// DeployOpts holds the parameters for redeploying a project to a running VM.
type DeployOpts struct {
	// Host is the external IP address of the VM.
	Host string

	// PrivateKeyPath is the path to the SSH private key file.
	PrivateKeyPath string

	// SSHUser is the username to connect with.
	SSHUser string

	// StorageEnabled + MountPath determine the compose directory, exactly as
	// during Provision (project lives on the persistent disk when enabled).
	StorageEnabled bool
	MountPath      string

	// ComposeContent is the pre-read content of the docker-compose.yml file.
	// Empty string means the compose file is left as-is on the VM.
	ComposeContent string

	// SyncDir is the local directory to re-sync to the VM (empty skips sync).
	SyncDir string
}

// ProvisionOpts holds the parameters needed to provision a VM.
type ProvisionOpts struct {
	// Host is the external IP address of the VM to provision.
	Host string

	// PrivateKeyPath is the path to the SSH private key file.
	PrivateKeyPath string

	// SSHUser is the username to connect with (e.g., "serverku" for GCP).
	SSHUser string

	// StorageEnabled indicates whether a persistent disk should be mounted.
	StorageEnabled bool

	// DiskName is the GCP disk resource name (e.g., "serverku-myproject-data").
	// Used to construct the stable device path /dev/disk/by-id/google-<DiskName>.
	// Only relevant when StorageEnabled is true.
	DiskName string

	// MountPath is the path inside the VM to mount the persistent disk
	// (e.g., "/data"). Only relevant when StorageEnabled is true.
	MountPath string

	// ComposeContent is the pre-read content of the docker-compose.yml file.
	// Empty string means no compose file should be deployed.
	ComposeContent string

	// SyncDir is the path to a local directory to synchronize to the remote VM.
	SyncDir string

	// RouterEnabled indicates whether to install caddyku and configure routing.
	RouterEnabled bool

	// Domains is a list of domains to configure via caddyku (only relevant if RouterEnabled is true).
	Domains []config.DomainConfig

	// StartupCommands are additional shell commands to run after provisioning.
	StartupCommands []string

	// Heartbeat configures the on-VM Telegram heartbeat. Heartbeat.Hours == 0
	// disables it.
	Heartbeat HeartbeatOpts
}

// HeartbeatOpts configures an on-VM still-running reminder: a systemd timer
// that messages the configured channels every Hours hours with uptime and
// accrued cost while the VM is running. It lives on the VM, so it keeps
// working when the local machine is offline and can never fire after the VM
// is destroyed.
type HeartbeatOpts struct {
	// Hours is the reminder interval; zero disables the heartbeat. When both
	// channels are configured with different intervals, the smaller wins.
	Hours int

	// ProjectName is included in the message and the suggested down command.
	ProjectName string

	// BotToken and ChatID enable the Telegram channel. The token is written
	// to a root-only script on the VM.
	BotToken string
	ChatID   string

	// NtfyServer and NtfyTopic enable the ntfy channel -- account-less push
	// where the topic name is the only capability, so nothing account-linked
	// lands on the VM.
	NtfyServer string
	NtfyTopic  string

	// HourlyRateUSD is the VM's hourly rate used to report accrued cost; zero
	// omits cost from the message. RateIsLive distinguishes a real provider
	// API price (shown as-is) from an offline table estimate (marked est.).
	HourlyRateUSD float64
	RateIsLive    bool
}

// enabled reports whether the heartbeat should be installed: an interval plus
// at least one configured channel.
func (hb HeartbeatOpts) enabled() bool {
	return hb.Hours > 0 && (hb.BotToken != "" && hb.ChatID != "" || hb.NtfyTopic != "")
}

// TeardownOpts holds the parameters needed to teardown a VM before destruction.
type TeardownOpts struct {
	// Host is the external IP address of the VM.
	Host string

	// PrivateKeyPath is the path to the SSH private key file.
	PrivateKeyPath string

	// SSHUser is the username to connect with.
	SSHUser string

	// StorageEnabled indicates whether a persistent disk is mounted and should
	// be unmounted.
	StorageEnabled bool

	// MountPath is the mount path of the persistent disk (e.g., "/data").
	// Only relevant when StorageEnabled is true.
	MountPath string

	// ComposeDir is the directory containing the docker-compose.yml on the VM.
	// Empty string means no compose was deployed (skip docker compose down).
	ComposeDir string
}

// NoopProvisioner is a provisioner that does nothing.
// Used when provisioning is not needed (e.g., in tests or when no compose file
// is configured).
type NoopProvisioner struct{}

func (n *NoopProvisioner) Provision(_ context.Context, _ ProvisionOpts) error { return nil }
func (n *NoopProvisioner) Deploy(_ context.Context, _ DeployOpts) error       { return nil }
func (n *NoopProvisioner) Teardown(_ context.Context, _ TeardownOpts) error   { return nil }

// SSHProvisioner is the concrete SSH-based implementation of Provisioner.
// It connects to a VM via SSH and runs setup scripts.
type SSHProvisioner struct{}

// Provision connects to the VM and runs the full setup sequence:
//  1. Wait for SSH (with retry)
//  2. Install Docker
//  3. Mount persistent disk (if storage enabled)
//  4. Transfer and start docker-compose (if compose content provided)
//  5. Run startup commands
func (p *SSHProvisioner) Provision(ctx context.Context, opts ProvisionOpts) error {
	log.Printf("[provisioner] connecting to %s as %s...", opts.Host, opts.SSHUser)

	client, err := connectSSHWithRetry(ctx, opts.Host, opts.PrivateKeyPath, opts.SSHUser)
	if err != nil {
		return fmt.Errorf("failed to connect to VM via SSH: %w", err)
	}
	defer client.Close()

	log.Printf("[provisioner] installing Docker...")
	if out, err := runCommand(client, installDockerScript); err != nil {
		log.Printf("[provisioner] docker install output:\n%s", out)
		return fmt.Errorf("failed to install Docker: %w", err)
	}
	log.Printf("[provisioner] Docker installed successfully")

	if opts.StorageEnabled {
		log.Printf("[provisioner] mounting disk %q at %s...", opts.DiskName, opts.MountPath)
		script := mountDiskScript(opts.DiskName, opts.MountPath)
		if out, err := runCommand(client, script); err != nil {
			log.Printf("[provisioner] disk mount output:\n%s", out)
			return fmt.Errorf("failed to mount disk: %w", err)
		}
		log.Printf("[provisioner] disk mounted at %s", opts.MountPath)
	}

	composeDir := "/home/" + opts.SSHUser
	if opts.StorageEnabled && opts.MountPath != "" {
		composeDir = opts.MountPath
	}

	if opts.SyncDir != "" {
		if err := rsyncDir(ctx, opts.SyncDir, opts.PrivateKeyPath, opts.SSHUser, opts.Host, composeDir); err != nil {
			return err
		}
	}

	if opts.ComposeContent != "" {
		log.Printf("[provisioner] writing docker-compose.yml to %s...", composeDir)
		if out, err := runCommand(client, writeComposeScript(opts.ComposeContent, composeDir)); err != nil {
			log.Printf("[provisioner] write compose output:\n%s", out)
			return fmt.Errorf("failed to write compose file: %w", err)
		}
	}

	if opts.RouterEnabled {
		log.Printf("[provisioner] installing caddyku...")
		if out, err := runCommand(client, installCaddykuScript()); err != nil {
			log.Printf("[provisioner] caddyku install output:\n%s", out)
			return fmt.Errorf("failed to install caddyku: %w", err)
		}

		log.Printf("[provisioner] initializing caddy proxy...")
		if out, err := runCommand(client, initCaddyProxyScript()); err != nil {
			log.Printf("[provisioner] caddy proxy init output:\n%s", out)
			return fmt.Errorf("failed to init caddy proxy: %w", err)
		}

		if len(opts.Domains) > 0 {
			var domains, services, upstreams []string
			for _, d := range opts.Domains {
				domains = append(domains, d.Domain)
				services = append(services, d.Service)
				upstreams = append(upstreams, d.Upstream)
			}
			log.Printf("[provisioner] configuring domains...")
			if out, err := runCommand(client, configureAppDomainsScript(domains, services, upstreams, composeDir)); err != nil {
				log.Printf("[provisioner] configure domains output:\n%s", out)
				return fmt.Errorf("failed to configure domains: %w", err)
			}
		}
	}

	if opts.ComposeContent != "" || opts.SyncDir != "" {
		log.Printf("[provisioner] running docker compose up -d...")
		if out, err := runCommand(client, composeUpScript(composeDir)); err != nil {
			log.Printf("[provisioner] docker compose up output:\n%s", out)
			return fmt.Errorf("failed to start containers: %w", err)
		}
		log.Printf("[provisioner] containers started")
	}

	if opts.Heartbeat.enabled() {
		log.Printf("[provisioner] installing heartbeat (every %dh)...", opts.Heartbeat.Hours)
		if out, err := runCommand(client, heartbeatScript(opts.Heartbeat)); err != nil {
			// Non-fatal: the deployment itself succeeded and the local
			// notifier still reports up/down. But warn loudly -- a silent
			// heartbeat failure means no still-running reminders.
			log.Printf("[provisioner] WARNING: heartbeat install failed (no still-running reminders will be sent): %v\noutput:\n%s", err, out)
		}
	}

	for i, cmd := range opts.StartupCommands {
		log.Printf("[provisioner] running startup command %d/%d: %s", i+1, len(opts.StartupCommands), cmd)
		if out, err := runCommand(client, cmd); err != nil {
			log.Printf("[provisioner] startup command output:\n%s", out)
			return fmt.Errorf("startup command %d failed: %w", i+1, err)
		}
	}

	log.Printf("[provisioner] provisioning complete")
	return nil
}

// Teardown connects to the VM and cleanly stops containers and unmounts storage.
// It uses a single-attempt SSH connection (not retry) because the VM may already
// be shutting down. Connection failure is treated as non-fatal.
func (p *SSHProvisioner) Teardown(ctx context.Context, opts TeardownOpts) error {
	if opts.Host == "" {
		log.Printf("[provisioner] teardown: no host IP, skipping")
		return nil
	}

	log.Printf("[provisioner] teardown: connecting to %s...", opts.Host)
	client, err := connectSSH(ctx, opts.Host, opts.PrivateKeyPath, opts.SSHUser)
	if err != nil {
		// Non-fatal: the VM may already be terminating.
		log.Printf("[provisioner] teardown: could not connect to VM (non-fatal): %v", err)
		return nil
	}
	defer client.Close()

	hasCompose := opts.ComposeDir != ""
	script := teardownScript(opts.ComposeDir, opts.MountPath, hasCompose, opts.StorageEnabled)

	log.Printf("[provisioner] teardown: running teardown script...")
	if out, err := runCommand(client, script); err != nil {
		// Also non-fatal: best effort.
		log.Printf("[provisioner] teardown: script warning (non-fatal): %v\nOutput:\n%s", err, out)
	}

	log.Printf("[provisioner] teardown complete")
	return nil
}

// rsyncDir pushes a local directory to destDir on the VM over rsync/SSH,
// excluding common build/VCS noise. Used by both Provision and Deploy.
func rsyncDir(ctx context.Context, syncDir, privateKeyPath, sshUser, host, destDir string) error {
	log.Printf("[provisioner] syncing directory %s to %s...", syncDir, destDir)

	rsyncBin, err := exec.LookPath("rsync")
	if err != nil {
		return fmt.Errorf("rsync not found in PATH. Please install rsync for directory synchronization: %w", err)
	}

	sshOpts := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=accept-new -o LogLevel=ERROR", privateKeyPath)
	dest := fmt.Sprintf("%s@%s:%s/", sshUser, host, destDir)

	rsyncCmd := exec.CommandContext(ctx, rsyncBin,
		"-avz",
		"-e", sshOpts,
		"--exclude=.git",
		"--exclude=node_modules",
		"--exclude=vendor",
		syncDir+"/", // Trailing slash to copy contents
		dest,
	)

	out, err := rsyncCmd.CombinedOutput()
	if err != nil {
		log.Printf("[provisioner] rsync output:\n%s", out)
		return fmt.Errorf("failed to sync directory: %w", err)
	}
	log.Printf("[provisioner] directory synced successfully")
	return nil
}

// Deploy pushes the current project state to an already-running VM: re-sync
// the project directory, rewrite the compose file, and `docker compose up -d`
// so changed services are recreated. Docker, the mounted disk, and Caddyku
// routing from the original Provision are reused untouched.
func (p *SSHProvisioner) Deploy(ctx context.Context, opts DeployOpts) error {
	log.Printf("[provisioner] deploy: connecting to %s as %s...", opts.Host, opts.SSHUser)

	client, err := connectSSHWithRetry(ctx, opts.Host, opts.PrivateKeyPath, opts.SSHUser)
	if err != nil {
		return fmt.Errorf("failed to connect to VM via SSH: %w", err)
	}
	defer client.Close()

	composeDir := "/home/" + opts.SSHUser
	if opts.StorageEnabled && opts.MountPath != "" {
		composeDir = opts.MountPath
	}

	if opts.SyncDir != "" {
		if err := rsyncDir(ctx, opts.SyncDir, opts.PrivateKeyPath, opts.SSHUser, opts.Host, composeDir); err != nil {
			return err
		}
	}

	if opts.ComposeContent != "" {
		log.Printf("[provisioner] deploy: writing docker-compose.yml to %s...", composeDir)
		if out, err := runCommand(client, writeComposeScript(opts.ComposeContent, composeDir)); err != nil {
			log.Printf("[provisioner] write compose output:\n%s", out)
			return fmt.Errorf("failed to write compose file: %w", err)
		}
	}

	log.Printf("[provisioner] deploy: running docker compose up -d...")
	if out, err := runCommand(client, composeUpScript(composeDir)); err != nil {
		log.Printf("[provisioner] docker compose up output:\n%s", out)
		return fmt.Errorf("failed to restart containers: %w", err)
	}

	log.Printf("[provisioner] deploy complete")
	return nil
}
