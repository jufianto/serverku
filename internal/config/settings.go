package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Settings holds CLI-wide preferences, separate from per-project cloud config.
type Settings struct {
	Debug bool `yaml:"debug"`
}

func (s *Store) SettingsPath() string {
	return filepath.Join(s.baseDir, "config.yaml")
}

// LoadSettings returns quiet defaults when the optional config file is absent.
func (s *Store) LoadSettings() (Settings, error) {
	var settings Settings
	data, err := os.ReadFile(s.SettingsPath())
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil {
		return settings, fmt.Errorf("failed to read CLI config %q: %w", s.SettingsPath(), err)
	}
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return settings, fmt.Errorf("invalid CLI config %q: %w", s.SettingsPath(), err)
	}
	return settings, nil
}
