package config

import (
	"testing"
)

// validGCPConfig returns a minimal valid GCP project config for testing.
func validGCPConfig(name string) *ProjectConfig {
	return &ProjectConfig{
		Name:      name,
		Provider:  "gcp",
		ProjectID: "test-gcp-project",
		Region:    "asia-southeast1",
		Zone:      "asia-southeast1-b",
		VM: VMConfig{
			Size: "e2-medium",
			Spot: true,
		},
	}
}

func TestProjectConfig_Validate_Valid(t *testing.T) {
	cfg := validGCPConfig("my-project")
	cfg.Storage = StorageConfig{
		Enabled:   true,
		SizeGB:    20,
		MountPath: "/data",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestProjectConfig_Validate_Stateless(t *testing.T) {
	cfg := validGCPConfig("stateless-api")
	cfg.Storage = StorageConfig{Enabled: false}

	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid stateless config, got error: %v", err)
	}
}

func TestProjectConfig_Validate_MissingName(t *testing.T) {
	cfg := validGCPConfig("")
	cfg.Name = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestProjectConfig_Validate_InvalidName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"my-project", false},
		{"project123", false},
		{"a", false},
		{"My-Project", true},     // uppercase
		{"-leading", true},       // leading hyphen
		{"trailing-", true},      // trailing hyphen
		{"has space", true},      // space
		{"has_underscore", true}, // underscore
		{"has.dot", true},        // dot
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validGCPConfig(tt.name)
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error for name %q", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for name %q: %v", tt.name, err)
			}
		})
	}
}

func TestProjectConfig_Validate_InvalidProvider(t *testing.T) {
	cfg := &ProjectConfig{
		Name:     "test",
		Provider: "aws",
		Region:   "us-east-1",
		VM:       VMConfig{Size: "t3.micro"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

func TestProjectConfig_Validate_GCPRequiresZone(t *testing.T) {
	cfg := validGCPConfig("test")
	cfg.Zone = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for GCP without zone")
	}
}

func TestProjectConfig_Validate_GCPRequiresProjectID(t *testing.T) {
	cfg := validGCPConfig("test")
	cfg.ProjectID = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for GCP without project_id")
	}
}

func TestProjectConfig_Validate_DONoZoneNeeded(t *testing.T) {
	cfg := &ProjectConfig{
		Name:     "test",
		Provider: "digitalocean",
		Region:   "sgp1",
		VM:       VMConfig{Size: "s-1vcpu-1gb"},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("DO should not require zone, got error: %v", err)
	}
}

func TestProjectConfig_Validate_StorageEnabled_MissingFields(t *testing.T) {
	cfg := validGCPConfig("test")
	cfg.Storage = StorageConfig{
		Enabled:   true,
		SizeGB:    0,
		MountPath: "",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for storage enabled with missing size and mount path")
	}
}

func TestProjectConfig_Validate_DNSRequiresDomains(t *testing.T) {
	cfg := validGCPConfig("test")
	cfg.DNS = DNSConfig{Enabled: true}

	if err := cfg.Validate(); err == nil {
		t.Error("expected error for dns enabled without router.domains")
	}

	// With a domain it should validate.
	cfg.Router = RouterConfig{
		Enabled: true,
		Domains: []DomainConfig{{Domain: "app.example.com", Service: "web", Upstream: "web:3000"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config with dns + domain, got: %v", err)
	}
}

func TestProjectConfig_SetDefaults_DNSTTL(t *testing.T) {
	cfg := validGCPConfig("test")
	cfg.DNS = DNSConfig{Enabled: true}

	cfg.SetDefaults()

	if cfg.DNS.TTL != 3600 {
		t.Errorf("expected default DNS TTL 3600, got %d", cfg.DNS.TTL)
	}

	// An explicit TTL is preserved.
	cfg2 := validGCPConfig("test")
	cfg2.DNS = DNSConfig{Enabled: true, TTL: 60}
	cfg2.SetDefaults()
	if cfg2.DNS.TTL != 60 {
		t.Errorf("expected explicit DNS TTL 60 to be preserved, got %d", cfg2.DNS.TTL)
	}
}

func TestProjectConfig_SetDefaults(t *testing.T) {
	cfg := validGCPConfig("test")
	cfg.VM.Image = ""
	cfg.Storage = StorageConfig{Enabled: true}

	cfg.SetDefaults()

	if cfg.VM.Image != "ubuntu-22-04" {
		t.Errorf("expected default image ubuntu-22-04, got %q", cfg.VM.Image)
	}
	if cfg.Storage.SizeGB != 20 {
		t.Errorf("expected default storage size 20, got %d", cfg.Storage.SizeGB)
	}
	if cfg.Storage.MountPath != "/data" {
		t.Errorf("expected default mount path /data, got %q", cfg.Storage.MountPath)
	}
}

func TestProjectState_IsRunning(t *testing.T) {
	tests := []struct {
		status ProjectStatus
		want   bool
	}{
		{StatusRunning, true},
		{StatusStarting, true},
		{StatusStopped, false},
		{StatusStopping, false},
		{StatusError, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			state := &ProjectState{Status: tt.status}
			if got := state.IsRunning(); got != tt.want {
				t.Errorf("IsRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewState(t *testing.T) {
	state := NewState("my-project", "gcp", "asia-southeast1", "asia-southeast1-b")

	if state.ProjectName != "my-project" {
		t.Errorf("expected project name my-project, got %q", state.ProjectName)
	}
	if state.Status != StatusStopped {
		t.Errorf("expected status stopped, got %q", state.Status)
	}
	if state.Provider != "gcp" {
		t.Errorf("expected provider gcp, got %q", state.Provider)
	}
}
