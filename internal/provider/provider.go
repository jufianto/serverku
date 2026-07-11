package provider

import "context"

// CloudProvider defines the interface that all cloud provider implementations must satisfy.
// Each cloud (GCP, DigitalOcean, etc.) implements this interface separately, allowing the
// orchestrator to remain cloud-agnostic.
type CloudProvider interface {
	// CreateVM creates a new virtual machine with the given configuration.
	// Returns the created VM metadata or an error.
	CreateVM(ctx context.Context, config VMConfig) (*VM, error)

	// DestroyVM permanently deletes a virtual machine by its ID.
	DestroyVM(ctx context.Context, vmID string) error

	// StartVM starts a previously stopped virtual machine.
	StartVM(ctx context.Context, vmID string) error

	// StopVM stops a running virtual machine without destroying it.
	StopVM(ctx context.Context, vmID string) error

	// GetVMStatus returns the current status of a virtual machine.
	GetVMStatus(ctx context.Context, vmID string) (*VMStatus, error)

	// GetExternalIP returns the external IP address of a running VM.
	GetExternalIP(ctx context.Context, vmID string) (string, error)

	// WaitForReady blocks until the VM is in a running state or the context is cancelled.
	WaitForReady(ctx context.Context, vmID string) error

	// CreateDisk creates a new persistent block storage disk.
	CreateDisk(ctx context.Context, config DiskConfig) (*Disk, error)

	// DeleteDisk permanently deletes a persistent disk.
	DeleteDisk(ctx context.Context, diskID string) error

	// AttachDisk attaches a persistent disk to a running VM.
	AttachDisk(ctx context.Context, vmID string, diskID string) error

	// DetachDisk detaches a persistent disk from a VM.
	DetachDisk(ctx context.Context, vmID string, diskID string) error

	// SnapshotDisk creates a snapshot of a persistent disk and returns the
	// provider-specific snapshot identifier.
	SnapshotDisk(ctx context.Context, diskName string, snapshotName string) (string, error)
}

// DNSManager is an optional capability implemented by cloud providers that can
// also manage DNS A records for the project's domains. The orchestrator detects
// support via a type assertion on the CloudProvider; providers without DNS
// automation simply do not implement it.
type DNSManager interface {
	// EnsureARecord creates or updates an A record for fqdn pointing at ip.
	// It is idempotent: an existing record with the same data is left unchanged.
	EnsureARecord(ctx context.Context, fqdn, ip string, ttl int) error
}

// FirewallManager is an optional capability implemented by cloud providers
// whose networks block inbound traffic unless a firewall rule explicitly
// allows it (e.g. GCP's default network). The orchestrator detects support
// via a type assertion on the CloudProvider; providers whose VMs are open by
// default (e.g. DigitalOcean) simply do not implement it.
type FirewallManager interface {
	// EnsureFirewall creates the project's firewall rule if it does not
	// already exist. The rule targets the project's network tags, so it is
	// created once and survives VM re-creation across down/up cycles.
	EnsureFirewall(ctx context.Context, projectName string) error

	// DeleteFirewall removes the project's firewall rule. Deleting a rule
	// that does not exist is not an error.
	DeleteFirewall(ctx context.Context, projectName string) error
}

// PriceCatalog is an optional capability implemented by cloud providers that
// can report real VM pricing from the cloud's own pricing API. Callers fall
// back to the offline pricing tables (clearly labeled as estimates) when the
// provider does not implement it or the lookup fails.
type PriceCatalog interface {
	// VMHourlyRateUSD returns the current hourly price for a machine size in
	// a region, honoring spot pricing when spot is true.
	VMHourlyRateUSD(ctx context.Context, size, region string, spot bool) (float64, error)
}

// UsageReporter is an optional capability implemented by cloud providers that
// can report real account-level month-to-date usage.
type UsageReporter interface {
	// MonthToDateUsageUSD returns the account's month-to-date usage in USD.
	MonthToDateUsageUSD(ctx context.Context) (float64, error)
}

// VMConfig holds provider-agnostic configuration for creating a VM.
type VMConfig struct {
	Name          string   // VM instance name
	Region        string   // Cloud region (e.g., "asia-southeast1", "sgp1")
	Zone          string   // Cloud zone (e.g., "asia-southeast1-b"), empty for providers without zones
	MachineType   string   // Machine type (e.g., "e2-medium", "s-1vcpu-1gb")
	Image         string   // OS image (e.g., "ubuntu-22-04")
	Spot          bool     // Use SPOT/preemptible instances
	Tags          []string // Network/firewall tags
	SSHPubKey     string   // SSH public key to inject into VM
	StartupScript string   // Script to run on first boot
	ProjectID     string   // Cloud project ID (GCP-specific, empty for others)
}

// VM represents a created virtual machine.
type VM struct {
	ID       string // Provider-specific VM identifier
	Name     string // VM instance name
	Zone     string // Zone where VM was created
	Provider string // Provider name (e.g., "gcp", "digitalocean")
}

// VMStatusState represents the possible states of a VM.
type VMStatusState string

const (
	VMStateRunning    VMStatusState = "running"
	VMStateStopped    VMStatusState = "stopped"
	VMStateStarting   VMStatusState = "starting"
	VMStateStopping   VMStatusState = "stopping"
	VMStateTerminated VMStatusState = "terminated"
	VMStateUnknown    VMStatusState = "unknown"
)

// VMStatus holds the current status of a virtual machine.
type VMStatus struct {
	ID         string        // Provider-specific VM identifier
	Name       string        // VM instance name
	State      VMStatusState // Current state
	ExternalIP string        // External IP if available
}

// DiskConfig holds provider-agnostic configuration for creating a persistent disk.
type DiskConfig struct {
	Name      string // Disk name
	Zone      string // Zone for the disk (must match VM zone)
	SizeGB    int64  // Disk size in gigabytes
	DiskType  string // Disk type (e.g., "pd-standard", "pd-ssd")
	ProjectID string // Cloud project ID (GCP-specific)
}

// Disk represents a created persistent disk.
type Disk struct {
	ID       string // Provider-specific disk identifier
	Name     string // Disk name
	Zone     string // Zone where disk was created
	SizeGB   int64  // Disk size in gigabytes
	Provider string // Provider name
}
