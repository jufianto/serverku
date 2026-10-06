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

	// CreateDiskFromSnapshot creates a new persistent disk restored from a
	// snapshot, referenced by its name (as printed by `serverku backup`) or
	// provider-specific ID.
	CreateDiskFromSnapshot(ctx context.Context, config DiskConfig, snapshot string) (*Disk, error)
}

// DiskAttacherByID lets providers attach the exact disk recorded in state,
// without rediscovering it through a potentially stale name listing. Providers
// that require disk names (such as GCP) continue using CloudProvider.AttachDisk.
type DiskAttacherByID interface {
	AttachDiskByID(ctx context.Context, vmName, diskID string) error
}

// SSHKeyManager tracks separately registered account keys before VM creation,
// so failed deployments can clean up keys they own without touching custom keys.
type SSHKeyManager interface {
	EnsureProjectSSHKey(ctx context.Context, projectName, publicKey string) (*SSHKey, error)
	DeleteSSHKey(ctx context.Context, id, publicKey string) error
}

type SSHKey struct {
	ID          string
	Name        string
	Fingerprint string
	Created     bool
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

// CredentialValidator is an optional capability implemented by cloud
// providers that can verify their credentials with a lightweight
// authenticated API call. `serverku check` uses it to catch bad or missing
// credentials before any billable resource is created.
type CredentialValidator interface {
	// ValidateCredentials makes a cheap authenticated request and returns an
	// error if the provider's credentials are missing, invalid, or lack
	// access.
	ValidateCredentials(ctx context.Context) error
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

// CatalogRegion is a region/location a provider offers, for `init` to present
// as a choice instead of a free-text field.
type CatalogRegion struct {
	Slug string // provider region slug, e.g. "sgp1" or "asia-southeast1"
	Name string // human-readable name, e.g. "Singapore 1" ("" if none)
}

// CatalogSize is a VM size/machine type a provider offers, with enough detail
// for `init` to show a meaningful, priced choice.
type CatalogSize struct {
	Slug         string  // size slug, e.g. "s-1vcpu-1gb" or "e2-medium"
	VCPUs        int     // number of vCPUs
	MemoryMB     int     // memory in MB
	DiskGB       int     // included disk in GB
	PriceMonthly float64 // USD/month (0 if unknown)
	PriceHourly  float64 // USD/hour (0 if unknown)
}

// CatalogLister is an optional capability implemented by providers that can
// enumerate their available regions and VM sizes from a live API, so `serverku
// init` can offer a selectable, priced list instead of a free-text field.
// Detection is via a type assertion on CloudProvider; when unavailable the CLI
// falls back to a curated static list.
type CatalogLister interface {
	// ListRegions returns the provider's available regions.
	ListRegions(ctx context.Context) ([]CatalogRegion, error)

	// ListSizes returns the available VM sizes, filtered to those offered in
	// the given region (empty region = no filter). Results are sorted cheapest
	// first.
	ListSizes(ctx context.Context, region string) ([]CatalogSize, error)
}

// ComponentQuery names the resources a caller wants ListComponents to look up.
// The caller supplies the project name, VM/disk names, and local SSH public key;
// each provider resolves the corresponding cloud resources.
type ComponentQuery struct {
	ProjectName string // e.g. "wpblog"
	VMName      string // e.g. "serverku-wpblog"
	DiskName    string // e.g. "serverku-wpblog-data" ("" when storage is disabled)
	SSHPubKey   string // local public key used during VM creation; never generated by inventory
	SSHKeyOwned bool   // registered by this project's lifecycle and removed by destroy
}

// Component describes one cloud resource serverku manages for a project, for
// the live inventory shown by `serverku status`.
type Component struct {
	Kind             string // "VM", "Volume", "SSH key", "Firewall", "Snapshot"
	Name             string // resource name/identifier ("" when not applicable)
	Detail           string // extra info (size, count) -- may be empty
	Present          bool   // whether the resource currently exists in the cloud
	RemovedByDestroy bool   // whether `serverku destroy` cleans it up automatically
	Shared           bool   // intentionally retained for use by multiple projects
	LookupFailed     bool   // presence is unknown because the lookup could not be completed
}

// ComponentLister is an optional capability implemented by cloud providers that
// can enumerate the resources serverku created for a project. `serverku status`
// uses it to show a live inventory and flag orphans (present resources that
// `destroy` does not remove). Detection is via a type assertion on
// CloudProvider; providers without it simply omit the Components section.
type ComponentLister interface {
	// ListComponents returns the live state of each serverku-managed resource
	// for the project. Per-resource lookup failures should degrade gracefully
	// (set LookupFailed when presence is unknown) rather than fail the whole call.
	ListComponents(ctx context.Context, q ComponentQuery) ([]Component, error)
}

// VMConfig holds provider-agnostic configuration for creating a VM.
type VMConfig struct {
	Name           string   // VM instance name
	Region         string   // Cloud region (e.g., "asia-southeast1", "sgp1")
	Zone           string   // Cloud zone (e.g., "asia-southeast1-b"), empty for providers without zones
	MachineType    string   // Machine type (e.g., "e2-medium", "s-1vcpu-1gb")
	Image          string   // OS image (e.g., "ubuntu-22-04")
	Spot           bool     // Use SPOT/preemptible instances
	MaxUptimeHours int      // Auto-delete the VM after N hours of runtime (0 = never); GCP only
	Tags           []string // Network/firewall tags
	SSHPubKey      string   // SSH public key to inject into VM
	SSHKeyID       string   // pre-registered account key, when supported by the provider
	StartupScript  string   // Script to run on first boot
	ProjectID      string   // Cloud project ID (GCP-specific, empty for others)
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
