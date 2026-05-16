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

	// Teardown stops running containers and unmounts storage before the VM
	// is destroyed.
	Teardown(ctx context.Context, opts TeardownOpts) error
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
		log.Printf("[provisioner] syncing directory %s to %s...", opts.SyncDir, composeDir)
		
		rsyncBin, err := exec.LookPath("rsync")
		if err != nil {
			return fmt.Errorf("rsync not found in PATH. Please install rsync for directory synchronization: %w", err)
		}

		sshOpts := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=accept-new -o LogLevel=ERROR", opts.PrivateKeyPath)
		dest := fmt.Sprintf("%s@%s:%s/", opts.SSHUser, opts.Host, composeDir)
		
		rsyncCmd := exec.CommandContext(ctx, rsyncBin,
			"-avz",
			"-e", sshOpts,
			"--exclude=.git",
			"--exclude=node_modules",
			"--exclude=vendor",
			opts.SyncDir+"/", // Trailing slash to copy contents
			dest,
		)
		
		out, err := rsyncCmd.CombinedOutput()
		if err != nil {
			log.Printf("[provisioner] rsync output:\n%s", out)
			return fmt.Errorf("failed to sync directory: %w", err)
		}
		log.Printf("[provisioner] directory synced successfully")
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
