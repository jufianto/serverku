package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Store provides read/write access to project configs and state files.
// Configs are stored as YAML in <baseDir>/projects/.
// State is stored as JSON in <baseDir>/state/.
// SSH keys are stored in <baseDir>/keys/.
type Store struct {
	baseDir string
}

// NewStore creates a new Store rooted at the given base directory.
// Typically this is ~/.serverku/.
func NewStore(baseDir string) (*Store, error) {
	s := &Store{baseDir: baseDir}
	if err := s.ensureDirs(); err != nil {
		return nil, fmt.Errorf("failed to initialize store at %s: %w", baseDir, err)
	}
	return s, nil
}

// DefaultBaseDir returns the default base directory (~/.serverku/).
func DefaultBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".serverku"), nil
}

// BaseDir returns the base directory of the store.
func (s *Store) BaseDir() string {
	return s.baseDir
}

// ProjectsDir returns the path to the projects config directory.
func (s *Store) ProjectsDir() string {
	return filepath.Join(s.baseDir, "projects")
}

// StateDir returns the path to the state directory.
func (s *Store) StateDir() string {
	return filepath.Join(s.baseDir, "state")
}

// KeysDir returns the path to the SSH keys directory.
func (s *Store) KeysDir() string {
	return filepath.Join(s.baseDir, "keys")
}

// CredentialsPath returns the path to the secrets file holding provider API
// tokens. It lives in the base dir (not projects/, which is shareable config)
// and is written with owner-only 0600 permissions.
func (s *Store) CredentialsPath() string {
	return filepath.Join(s.baseDir, "credentials.yaml")
}

// GcloudDir returns serverku's isolated gcloud config directory. `serverku
// setup gcp` writes Application Default Credentials here (via CLOUDSDK_CONFIG)
// so serverku's GCP credentials never touch the user's system-wide
// ~/.config/gcloud.
func (s *Store) GcloudDir() string {
	return filepath.Join(s.baseDir, "gcloud")
}

// GcloudADCPath returns the path to the ADC file inside serverku's isolated
// gcloud directory (created by `serverku setup gcp`).
func (s *Store) GcloudADCPath() string {
	return filepath.Join(s.GcloudDir(), "application_default_credentials.json")
}

// SaveCredential stores a provider API token in the 0600 credentials file,
// merging with any existing entries. The file mode is re-asserted on every
// write so an existing loosely-permissioned file is tightened.
func (s *Store) SaveCredential(provider, token string) error {
	creds, err := s.loadCredentials()
	if err != nil {
		return err
	}
	creds[provider] = token

	data, err := yaml.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to encode credentials: %w", err)
	}
	path := s.CredentialsPath()
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}
	// WriteFile only applies the mode when creating; enforce it on an
	// existing file too so the secret is never left world-readable.
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("failed to secure credentials file: %w", err)
	}
	return nil
}

// LoadCredential returns the stored token for a provider, or "" if the
// credentials file or that entry does not exist.
func (s *Store) LoadCredential(provider string) (string, error) {
	creds, err := s.loadCredentials()
	if err != nil {
		return "", err
	}
	return creds[provider], nil
}

// loadCredentials reads the credentials file into a map, returning an empty
// map when the file does not exist.
func (s *Store) loadCredentials() (map[string]string, error) {
	data, err := os.ReadFile(s.CredentialsPath())
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials: %w", err)
	}
	creds := map[string]string{}
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}
	if creds == nil {
		creds = map[string]string{}
	}
	return creds, nil
}

// ensureDirs creates the directory structure if it doesn't exist.
func (s *Store) ensureDirs() error {
	dirs := []string{
		s.ProjectsDir(),
		s.StateDir(),
		s.KeysDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// SaveProject writes a ProjectConfig to disk as YAML.
func (s *Store) SaveProject(config *ProjectConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}

	path := filepath.Join(s.ProjectsDir(), config.Name+".yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write project config: %w", err)
	}

	return nil
}

// LoadProject reads a ProjectConfig from disk by project name.
func (s *Store) LoadProject(name string) (*ProjectConfig, error) {
	path := filepath.Join(s.ProjectsDir(), name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project %q not found", name)
		}
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	var config ProjectConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse project config: %w", err)
	}

	return &config, nil
}

// DeleteProject removes a project config file from disk.
func (s *Store) DeleteProject(name string) error {
	path := filepath.Join(s.ProjectsDir(), name+".yaml")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // already gone
		}
		return fmt.Errorf("failed to delete project config: %w", err)
	}
	return nil
}

// ProjectExists returns true if a project config file exists.
func (s *Store) ProjectExists(name string) bool {
	path := filepath.Join(s.ProjectsDir(), name+".yaml")
	_, err := os.Stat(path)
	return err == nil
}

// ListProjects returns the names of all project configs on disk.
func (s *Store) ListProjects() ([]string, error) {
	entries, err := os.ReadDir(s.ProjectsDir())
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") {
			names = append(names, strings.TrimSuffix(name, ".yaml"))
		}
	}

	return names, nil
}

// SaveState writes a ProjectState to disk as JSON.
func (s *Store) SaveState(state *ProjectState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project state: %w", err)
	}

	path := filepath.Join(s.StateDir(), state.ProjectName+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write project state: %w", err)
	}

	return nil
}

// LoadState reads a ProjectState from disk by project name.
// Returns a new empty state if the state file doesn't exist.
func (s *Store) LoadState(name string) (*ProjectState, error) {
	path := filepath.Join(s.StateDir(), name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectState{
				ProjectName: name,
				Status:      StatusStopped,
			}, nil
		}
		return nil, fmt.Errorf("failed to read project state: %w", err)
	}

	var state ProjectState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse project state: %w", err)
	}

	return &state, nil
}

// DeleteState removes a project state file from disk.
func (s *Store) DeleteState(name string) error {
	path := filepath.Join(s.StateDir(), name+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete project state: %w", err)
	}
	return nil
}
