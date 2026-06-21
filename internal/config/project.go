package config

import (
	"fmt"
	"strings"
	"time"
)

// ProjectConfig represents the user-editable project configuration stored as YAML.
// Each project has one config file at ~/.serverku/projects/<name>.yaml.
type ProjectConfig struct {
	Name      string        `yaml:"name"`
	Provider  string        `yaml:"provider"`             // "gcp" or "digitalocean"
	ProjectID string        `yaml:"project_id,omitempty"` // Cloud project ID (required for GCP)
	Region    string        `yaml:"region"`
	Zone      string        `yaml:"zone,omitempty"` // GCP requires zone, DO does not
	VM        VMConfig      `yaml:"vm"`
	Storage   StorageConfig `yaml:"storage"`

	// ComposeFile is the path to the docker-compose.yml file.
	// Can be relative (to the project config directory) or absolute.
	ComposeFile string `yaml:"compose_file,omitempty"`

	// SyncDir is the path to a local directory to synchronize to the remote VM.
	SyncDir string `yaml:"sync_dir,omitempty"`

	// StartupCommands are shell commands to run after provisioning is complete.
	StartupCommands []string `yaml:"startup_commands,omitempty"`

	// Router holds automatic HTTPS routing and reverse proxy configuration via caddyku.
	Router RouterConfig `yaml:"router,omitempty"`

	// DNS holds automatic DNS record management settings.
	DNS DNSConfig `yaml:"dns,omitempty"`

	// Notifications holds optional notification settings.
	Notifications NotificationsConfig `yaml:"notifications,omitempty"`
}

// DNSConfig holds automatic DNS record management settings. When enabled,
// serverku creates/updates an A record for each domain in router.domains
// pointing at the VM's external IP on `up`.
type DNSConfig struct {
	Enabled bool `yaml:"enabled"`
	TTL     int  `yaml:"ttl,omitempty"` // record TTL in seconds, defaults to 3600
}

// NotificationsConfig holds notification settings.
type NotificationsConfig struct {
	Slack    SlackConfig    `yaml:"slack,omitempty"`
	Telegram TelegramConfig `yaml:"telegram,omitempty"`
}

// RouterConfig holds caddyku routing configuration.
type RouterConfig struct {
	Enabled bool           `yaml:"enabled"`
	Domains []DomainConfig `yaml:"domains,omitempty"`
}

// DomainConfig holds routing configuration for a specific domain.
type DomainConfig struct {
	Domain   string `yaml:"domain"`
	Service  string `yaml:"service"`
	Upstream string `yaml:"upstream"`
}

// SlackConfig holds Slack notification settings.
type SlackConfig struct {
	WebhookURL string `yaml:"webhook_url,omitempty"`
}

// TelegramConfig holds Telegram notification settings.
type TelegramConfig struct {
	BotToken string `yaml:"bot_token,omitempty"`
	ChatID   string `yaml:"chat_id,omitempty"`
}

// VMConfig holds VM-specific configuration.
type VMConfig struct {
	Size  string `yaml:"size"`            // Machine type (e.g., "e2-medium", "s-1vcpu-1gb")
	Image string `yaml:"image,omitempty"` // OS image, defaults to "ubuntu-22-04"
	Spot  bool   `yaml:"spot"`            // Use SPOT/preemptible instances
}

// StorageConfig holds persistent block storage configuration.
type StorageConfig struct {
	Enabled   bool   `yaml:"enabled"`              // Whether to use persistent storage
	SizeGB    int    `yaml:"size_gb,omitempty"`    // Disk size in GB
	MountPath string `yaml:"mount_path,omitempty"` // Mount path inside VM (e.g., "/data")
}

// NotifyConfig holds notification settings.
type NotifyConfig struct {
	TelegramChatID    string `yaml:"telegram_chat_id,omitempty"`
	BillingAlertHours int    `yaml:"billing_alert_hours,omitempty"` // Alert if VM running > N hours
	DailySummary      bool   `yaml:"daily_summary,omitempty"`       // Send daily cost summary
}

// Validate checks the project config for required fields and valid values.
func (c *ProjectConfig) Validate() error {
	var errs []string

	if c.Name == "" {
		errs = append(errs, "name is required")
	}

	if c.Name != "" && !isValidName(c.Name) {
		errs = append(errs, "name must contain only lowercase letters, numbers, and hyphens")
	}

	if c.Provider == "" {
		errs = append(errs, "provider is required")
	} else if c.Provider != "gcp" && c.Provider != "digitalocean" {
		errs = append(errs, fmt.Sprintf("unsupported provider %q, must be \"gcp\" or \"digitalocean\"", c.Provider))
	}

	if c.Region == "" {
		errs = append(errs, "region is required")
	}

	if c.Provider == "gcp" && c.Zone == "" {
		errs = append(errs, "zone is required for GCP provider")
	}

	if c.Provider == "gcp" && c.ProjectID == "" {
		errs = append(errs, "project_id is required for GCP provider")
	}

	if c.VM.Size == "" {
		errs = append(errs, "vm.size is required")
	}

	if c.Storage.Enabled {
		if c.Storage.SizeGB <= 0 {
			errs = append(errs, "storage.size_gb must be > 0 when storage is enabled")
		}
		if c.Storage.MountPath == "" {
			errs = append(errs, "storage.mount_path is required when storage is enabled")
		}
	}

	if c.DNS.Enabled && len(c.Router.Domains) == 0 {
		errs = append(errs, "dns.enabled requires at least one router.domains entry")
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid project config:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// SetDefaults fills in default values for optional fields.
func (c *ProjectConfig) SetDefaults() {
	if c.VM.Image == "" {
		c.VM.Image = "ubuntu-22-04"
	}
	if c.Storage.Enabled && c.Storage.MountPath == "" {
		c.Storage.MountPath = "/data"
	}
	if c.Storage.Enabled && c.Storage.SizeGB == 0 {
		c.Storage.SizeGB = 20
	}
	if c.DNS.Enabled && c.DNS.TTL == 0 {
		c.DNS.TTL = 3600
	}
}

// isValidName checks that a project name contains only lowercase letters, numbers, and hyphens.
func isValidName(name string) bool {
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return len(name) > 0 && name[0] != '-' && name[len(name)-1] != '-'
}

// ProjectStatus represents the lifecycle status of a project.
type ProjectStatus string

const (
	StatusStopped  ProjectStatus = "stopped"
	StatusStarting ProjectStatus = "starting"
	StatusRunning  ProjectStatus = "running"
	StatusStopping ProjectStatus = "stopping"
	StatusError    ProjectStatus = "error"
)

// ProjectState holds the runtime state of a project, managed by serverku.
// Stored as JSON at ~/.serverku/state/<name>.json.
type ProjectState struct {
	ProjectName string        `json:"project_name"`
	Status      ProjectStatus `json:"status"`
	VMID        string        `json:"vm_id,omitempty"`
	VMName      string        `json:"vm_name,omitempty"` // Instance name (used by GCP for API calls)
	DiskID      string        `json:"disk_id,omitempty"`
	DiskName    string        `json:"disk_name,omitempty"` // Disk name (used by GCP for API calls)
	ExternalIP  string        `json:"external_ip,omitempty"`
	Provider    string        `json:"provider"`
	Region      string        `json:"region"`
	Zone        string        `json:"zone,omitempty"`
	CreatedAt   *time.Time    `json:"created_at,omitempty"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	StoppedAt   *time.Time    `json:"stopped_at,omitempty"`
	ErrorMsg    string        `json:"error_msg,omitempty"`
}

// IsRunning returns true if the project has a VM currently running.
func (s *ProjectState) IsRunning() bool {
	return s.Status == StatusRunning || s.Status == StatusStarting
}

// NewState creates a new ProjectState with default values.
func NewState(projectName string, provider string, region string, zone string) *ProjectState {
	return &ProjectState{
		ProjectName: projectName,
		Status:      StatusStopped,
		Provider:    provider,
		Region:      region,
		Zone:        zone,
	}
}
