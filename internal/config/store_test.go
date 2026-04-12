package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return s
}

func TestNewStore_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	for _, d := range []string{s.ProjectsDir(), s.StateDir(), s.KeysDir()} {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("directory %s not created: %v", d, err)
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
	}
}

func TestStore_SaveAndLoadProject(t *testing.T) {
	s := newTestStore(t)

	cfg := &ProjectConfig{
		Name:     "test-project",
		Provider: "gcp",
		Region:   "asia-southeast1",
		Zone:     "asia-southeast1-b",
		VM: VMConfig{
			Size:  "e2-medium",
			Image: "ubuntu-22-04",
			Spot:  true,
		},
		Storage: StorageConfig{
			Enabled:   true,
			SizeGB:    20,
			MountPath: "/data",
		},
	}

	// Save
	if err := s.SaveProject(cfg); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(s.ProjectsDir(), "test-project.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Verify file is valid YAML
	data, _ := os.ReadFile(path)
	var parsed ProjectConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("saved file is not valid YAML: %v", err)
	}

	// Load
	loaded, err := s.LoadProject("test-project")
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if loaded.Name != cfg.Name {
		t.Errorf("name mismatch: got %q, want %q", loaded.Name, cfg.Name)
	}
	if loaded.Provider != cfg.Provider {
		t.Errorf("provider mismatch: got %q, want %q", loaded.Provider, cfg.Provider)
	}
	if loaded.VM.Size != cfg.VM.Size {
		t.Errorf("vm size mismatch: got %q, want %q", loaded.VM.Size, cfg.VM.Size)
	}
	if loaded.Storage.SizeGB != cfg.Storage.SizeGB {
		t.Errorf("storage size mismatch: got %d, want %d", loaded.Storage.SizeGB, cfg.Storage.SizeGB)
	}
	if loaded.VM.Spot != cfg.VM.Spot {
		t.Errorf("spot mismatch: got %v, want %v", loaded.VM.Spot, cfg.VM.Spot)
	}
}

func TestStore_LoadProject_NotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.LoadProject("nonexistent")
	if err == nil {
		t.Error("expected error loading nonexistent project")
	}
}

func TestStore_DeleteProject(t *testing.T) {
	s := newTestStore(t)

	cfg := &ProjectConfig{Name: "to-delete", Provider: "gcp", Region: "us-central1", Zone: "us-central1-a", VM: VMConfig{Size: "e2-small"}}
	s.SaveProject(cfg)

	if err := s.DeleteProject("to-delete"); err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}

	if s.ProjectExists("to-delete") {
		t.Error("project should not exist after deletion")
	}
}

func TestStore_DeleteProject_NotFound(t *testing.T) {
	s := newTestStore(t)

	// Should not error when deleting nonexistent project
	if err := s.DeleteProject("nonexistent"); err != nil {
		t.Errorf("DeleteProject should not error for nonexistent: %v", err)
	}
}

func TestStore_ProjectExists(t *testing.T) {
	s := newTestStore(t)

	if s.ProjectExists("nope") {
		t.Error("expected false for nonexistent project")
	}

	cfg := &ProjectConfig{Name: "exists", Provider: "gcp", Region: "us-central1", Zone: "us-central1-a", VM: VMConfig{Size: "e2-small"}}
	s.SaveProject(cfg)

	if !s.ProjectExists("exists") {
		t.Error("expected true for existing project")
	}
}

func TestStore_ListProjects(t *testing.T) {
	s := newTestStore(t)

	// Empty
	names, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 projects, got %d", len(names))
	}

	// Add some projects
	for _, name := range []string{"alpha", "beta", "gamma"} {
		cfg := &ProjectConfig{Name: name, Provider: "gcp", Region: "us-central1", Zone: "us-central1-a", VM: VMConfig{Size: "e2-small"}}
		s.SaveProject(cfg)
	}

	names, err = s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(names) != 3 {
		t.Errorf("expected 3 projects, got %d", len(names))
	}
}

func TestStore_SaveAndLoadState(t *testing.T) {
	s := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	state := &ProjectState{
		ProjectName: "my-project",
		Status:      StatusRunning,
		VMID:        "12345",
		DiskID:      "serverku-my-project-data",
		ExternalIP:  "34.101.123.45",
		Provider:    "gcp",
		Region:      "asia-southeast1",
		Zone:        "asia-southeast1-b",
		StartedAt:   &now,
	}

	// Save
	if err := s.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Verify file is valid JSON
	path := filepath.Join(s.StateDir(), "my-project.json")
	data, _ := os.ReadFile(path)
	var parsed ProjectState
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("saved state is not valid JSON: %v", err)
	}

	// Load
	loaded, err := s.LoadState("my-project")
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if loaded.ProjectName != "my-project" {
		t.Errorf("project name mismatch: got %q", loaded.ProjectName)
	}
	if loaded.Status != StatusRunning {
		t.Errorf("status mismatch: got %q", loaded.Status)
	}
	if loaded.VMID != "12345" {
		t.Errorf("vm id mismatch: got %q", loaded.VMID)
	}
	if loaded.ExternalIP != "34.101.123.45" {
		t.Errorf("ip mismatch: got %q", loaded.ExternalIP)
	}
}

func TestStore_LoadState_NotFound_ReturnsDefaultState(t *testing.T) {
	s := newTestStore(t)

	state, err := s.LoadState("nonexistent")
	if err != nil {
		t.Fatalf("LoadState should not error for nonexistent: %v", err)
	}

	if state.ProjectName != "nonexistent" {
		t.Errorf("expected project name nonexistent, got %q", state.ProjectName)
	}
	if state.Status != StatusStopped {
		t.Errorf("expected default status stopped, got %q", state.Status)
	}
}

func TestStore_DeleteState(t *testing.T) {
	s := newTestStore(t)

	state := &ProjectState{ProjectName: "to-delete", Status: StatusStopped}
	s.SaveState(state)

	if err := s.DeleteState("to-delete"); err != nil {
		t.Fatalf("DeleteState failed: %v", err)
	}

	// After delete, LoadState should return default
	loaded, _ := s.LoadState("to-delete")
	if loaded.VMID != "" {
		t.Error("expected empty VM ID after state deletion")
	}
}
