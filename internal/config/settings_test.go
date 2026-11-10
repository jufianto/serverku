package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadSettings(t *testing.T) {
	for _, tc := range []struct {
		name, content  string
		debug, invalid bool
	}{
		{name: "missing"},
		{name: "enabled", content: "debug: true\n", debug: true},
		{name: "disabled", content: "debug: false\n"},
		{name: "invalid boolean", content: "debug: invalid\n", invalid: true},
		{name: "invalid YAML", content: "debug: [\n", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := NewStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if tc.content != "" {
				if err := os.WriteFile(s.SettingsPath(), []byte(tc.content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			settings, err := s.LoadSettings()
			if tc.invalid {
				if err == nil || !strings.Contains(err.Error(), s.SettingsPath()) {
					t.Fatalf("invalid config error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if settings.Debug != tc.debug {
				t.Errorf("Debug = %v, want %v", settings.Debug, tc.debug)
			}
		})
	}
}
