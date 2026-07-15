package config

import (
	"crypto/rand"
	"encoding/hex"
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

	// Hooks holds shell commands run on the LOCAL machine at lifecycle boundaries.
	Hooks HooksConfig `yaml:"hooks,omitempty"`

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

// HooksConfig holds shell commands run on the local machine (not the VM) around
// lifecycle operations. pre_* hooks gate the operation: if one fails, the
// operation is aborted. post_* hooks run after a successful operation and are
// best-effort (a failure is logged but does not fail the command).
type HooksConfig struct {
	PreUp       []string `yaml:"pre_up,omitempty"`
	PostUp      []string `yaml:"post_up,omitempty"`
	PreDeploy   []string `yaml:"pre_deploy,omitempty"`
	PostDeploy  []string `yaml:"post_deploy,omitempty"`
	PreDown     []string `yaml:"pre_down,omitempty"`
	PostDown    []string `yaml:"post_down,omitempty"`
	PreDestroy  []string `yaml:"pre_destroy,omitempty"`
	PostDestroy []string `yaml:"post_destroy,omitempty"`
}

// NotificationsConfig holds notification settings.
type NotificationsConfig struct {
	Slack    SlackConfig    `yaml:"slack,omitempty"`
	Telegram TelegramConfig `yaml:"telegram,omitempty"`
	Ntfy     NtfyConfig     `yaml:"ntfy,omitempty"`
}

// NtfyConfig holds ntfy (https://ntfy.sh) notification settings. ntfy is
// account-less publish/subscribe push: the topic name is the only
// capability, so it is safe to place on the VM for heartbeats -- worst case
// someone who learns it can send notifications to that one topic.
type NtfyConfig struct {
	// Server is the ntfy server base URL. Empty means https://ntfy.sh.
	Server string `yaml:"server,omitempty"`

	// Topic is the topic to publish to. `serverku init` generates a random
	// unguessable one (serverku-<project>-<random>). Subscribe to it in the
	// ntfy app; see `serverku ntfy <project>`.
	Topic string `yaml:"topic,omitempty"`

	// HeartbeatHours enables the on-VM still-running reminder via ntfy,
	// like notifications.telegram.heartbeat_hours but with no account-linked
	// secret on the VM. Zero disables it. Requires topic.
	HeartbeatHours int `yaml:"heartbeat_hours,omitempty"`
}

// DefaultNtfyServer is the public ntfy server used when server is unset.
const DefaultNtfyServer = "https://ntfy.sh"

// ServerURL returns the configured ntfy server or the public default.
func (n NtfyConfig) ServerURL() string {
	if n.Server != "" {
		return strings.TrimRight(n.Server, "/")
	}
	return DefaultNtfyServer
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

	// HeartbeatHours enables an on-VM reminder: while the VM runs, a systemd
	// timer on the VM messages the chat every N hours with uptime and accrued
	// cost, so a forgotten VM cannot burn budget silently. It runs on the VM
	// itself (works while the local machine is off) and dies with the VM.
	// Zero disables it. Requires bot_token and chat_id.
	HeartbeatHours int `yaml:"heartbeat_hours,omitempty"`
}

// VMConfig holds VM-specific configuration.
type VMConfig struct {
	Size  string `yaml:"size"`            // Machine type (e.g., "e2-medium", "s-1vcpu-1gb")
	Image string `yaml:"image,omitempty"` // OS image, defaults to "ubuntu-22-04"
	Spot  bool   `yaml:"spot"`            // Use SPOT/preemptible instances

	// MaxUptimeHours auto-deletes the VM after this many hours of runtime as
	// a hard budget cap (the disk survives, so it's like an automatic
	// `serverku down`). Enforced by the cloud itself, not serverku, so it
	// fires even if your machine is off. GCP only -- DigitalOcean has no
	// equivalent. Zero disables it.
	MaxUptimeHours int `yaml:"max_uptime_hours,omitempty"`
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

	if c.Provider == "digitalocean" && c.VM.Spot {
		errs = append(errs, "vm.spot is not supported on digitalocean (GCP only)")
	}

	if hb := c.VM.MaxUptimeHours; hb != 0 {
		if hb < 0 || hb > 168 {
			errs = append(errs, "vm.max_uptime_hours must be between 1 and 168")
		}
		if c.Provider == "digitalocean" {
			errs = append(errs, "vm.max_uptime_hours is not supported on digitalocean (GCP only; a powered-off droplet still bills, so there is no safe auto-shutdown -- use notifications.*.heartbeat_hours as a reminder instead)")
		}
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

	if hb := c.Notifications.Telegram.HeartbeatHours; hb != 0 {
		if hb < 0 || hb > 168 {
			errs = append(errs, "notifications.telegram.heartbeat_hours must be between 1 and 168")
		}
		if c.Notifications.Telegram.BotToken == "" || c.Notifications.Telegram.ChatID == "" {
			errs = append(errs, "notifications.telegram.heartbeat_hours requires bot_token and chat_id")
		}
	}

	if hb := c.Notifications.Ntfy.HeartbeatHours; hb != 0 {
		if hb < 0 || hb > 168 {
			errs = append(errs, "notifications.ntfy.heartbeat_hours must be between 1 and 168")
		}
		if c.Notifications.Ntfy.Topic == "" {
			errs = append(errs, "notifications.ntfy.heartbeat_hours requires topic")
		}
	}
	if topic := c.Notifications.Ntfy.Topic; topic != "" && !isValidNtfyTopic(topic) {
		errs = append(errs, "notifications.ntfy.topic must contain only letters, digits, underscores, and hyphens (max 64 chars)")
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

// isValidNtfyTopic checks that an ntfy topic is a valid topic name: letters,
// digits, underscores, and hyphens, at most 64 characters.
func isValidNtfyTopic(topic string) bool {
	if len(topic) == 0 || len(topic) > 64 {
		return false
	}
	for _, r := range topic {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// GenerateNtfyTopic returns a random unguessable ntfy topic for a project:
// serverku-<project>-<12 hex chars>. The randomness is the capability -- the
// topic name is the only thing needed to publish to (or read) the topic.
func GenerateNtfyTopic(projectName string) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random topic: %w", err)
	}
	topic := fmt.Sprintf("serverku-%s-%s", projectName, hex.EncodeToString(b))
	if len(topic) > 64 {
		topic = topic[:64]
	}
	return topic, nil
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
