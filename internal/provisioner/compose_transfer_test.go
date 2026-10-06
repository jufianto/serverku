package provisioner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWriteComposeScriptRoundTrip(t *testing.T) {
	// Match files saved without a trailing newline, including Compose variables
	// that the transfer must preserve instead of expanding in the remote shell.
	content := "services:\n  kuma:\n    image: louislam/uptime-kuma:1\n    environment:\n      - TOKEN=${TOKEN}\n    ports:\n      - \"3001:3001\""
	for _, suffix := range []string{"", "\n"} {
		name := "without trailing newline"
		if suffix != "" {
			name = "with trailing newline"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			// Execute the actual script without privileges: ignore ownership changes
			// and forward mkdir, which only writes inside this test's temporary dir.
			sudo := filepath.Join(dir, "sudo")
			if err := os.WriteFile(sudo, []byte("#!/bin/sh\nif [ \"$1\" = chown ]; then exit 0; fi\nexec \"$@\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "project")
			cmd := exec.Command("sh")
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "TOKEN=must-not-expand")
			cmd.Stdin = strings.NewReader(writeComposeScript(content+suffix, target))
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("transfer failed: %v\n%s", err, output)
			}
			got, err := os.ReadFile(filepath.Join(target, "docker-compose.yml"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != content+"\n" {
				t.Fatalf("transferred content differs:\n%s", got)
			}
			var doc map[string]any
			if err := yaml.Unmarshal(got, &doc); err != nil {
				t.Fatalf("transferred Compose is invalid YAML: %v", err)
			}
		})
	}
}
