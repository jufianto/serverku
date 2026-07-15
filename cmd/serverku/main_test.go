package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binPath is the compiled serverku binary, built once in TestMain. These are
// end-to-end tests that exercise the real CLI (flag parsing, validation, store
// persistence) against a throwaway --config-dir. Only offline commands are
// covered; up/down/status/ssh require cloud or SSH access.
var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "serverku-bin-")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "serverku")

	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("failed to build serverku binary: " + err.Error())
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// runCLI runs the built binary with the given args and returns stdout, stderr,
// and the exit error (nil on success).
func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	return out.String(), errOut.String(), err
}

// initDO runs a valid non-interactive DigitalOcean init in the given config dir.
func initDO(t *testing.T, cfgDir, name string) (string, string, error) {
	t.Helper()
	return runCLI(t, "--config-dir", cfgDir, "init", name,
		"--non-interactive", "--provider", "digitalocean",
		"--region", "sgp1", "--size", "s-1vcpu-1gb", "--no-storage")
}

func TestVersion(t *testing.T) {
	out, _, err := runCLI(t, "--version")
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(out, "serverku version") {
		t.Errorf("unexpected version output: %q", out)
	}
}

func TestHelpListsCommands(t *testing.T) {
	out, _, err := runCLI(t, "--help")
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	for _, want := range []string{"init", "up", "down", "status", "list", "backup"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing command %q\n%s", want, out)
		}
	}
}

func TestInitWritesConfigAndAppearsInList(t *testing.T) {
	cfgDir := t.TempDir()

	out, errOut, err := initDO(t, cfgDir, "demo")
	if err != nil {
		t.Fatalf("init failed: %v\nstderr: %s", err, errOut)
	}
	if !strings.Contains(out, "demo") {
		t.Errorf("init output missing project name: %q", out)
	}

	// Config file is persisted under the chosen config dir.
	cfgPath := filepath.Join(cfgDir, "projects", "demo.yaml")
	if _, statErr := os.Stat(cfgPath); statErr != nil {
		t.Fatalf("expected config at %s: %v", cfgPath, statErr)
	}

	// SSH keypair is generated.
	if _, statErr := os.Stat(filepath.Join(cfgDir, "keys", "serverku_rsa")); statErr != nil {
		t.Errorf("expected SSH private key to be generated: %v", statErr)
	}

	// list shows the project with provider and default stopped state.
	listOut, _, listErr := runCLI(t, "--config-dir", cfgDir, "list")
	if listErr != nil {
		t.Fatalf("list failed: %v", listErr)
	}
	if !strings.Contains(listOut, "demo") || !strings.Contains(listOut, "digitalocean") {
		t.Errorf("list missing project/provider: %q", listOut)
	}
	if !strings.Contains(listOut, "stopped") {
		t.Errorf("expected default stopped status in list: %q", listOut)
	}
}

func TestInitDuplicateFails(t *testing.T) {
	cfgDir := t.TempDir()

	if _, errOut, err := initDO(t, cfgDir, "demo"); err != nil {
		t.Fatalf("first init failed: %v\nstderr: %s", err, errOut)
	}

	_, errOut, err := initDO(t, cfgDir, "demo")
	if err == nil {
		t.Fatal("expected duplicate init to fail, got success")
	}
	if !strings.Contains(errOut, "already exists") {
		t.Errorf("expected 'already exists' error, got: %q", errOut)
	}
}

func TestInitInvalidProviderFails(t *testing.T) {
	cfgDir := t.TempDir()

	_, errOut, err := runCLI(t, "--config-dir", cfgDir, "init", "demo",
		"--non-interactive", "--provider", "aws", "--region", "us-east-1")
	if err == nil {
		t.Fatal("expected invalid provider to fail, got success")
	}
	if !strings.Contains(errOut, "unsupported provider") {
		t.Errorf("expected 'unsupported provider' error, got: %q", errOut)
	}
}

func TestInitDODefaultsSpotOff(t *testing.T) {
	cfgDir := t.TempDir()

	// Without an explicit --spot, DigitalOcean init must not inherit the
	// GCP-oriented spot default (DO has no spot equivalent and would fail at up).
	if _, errOut, err := initDO(t, cfgDir, "demo"); err != nil {
		t.Fatalf("init failed: %v\nstderr: %s", err, errOut)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "projects", "demo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "spot: true") {
		t.Errorf("DO config should not have spot: true:\n%s", data)
	}
}

func TestInitDOExplicitSpotFails(t *testing.T) {
	cfgDir := t.TempDir()

	_, errOut, err := runCLI(t, "--config-dir", cfgDir, "init", "demo",
		"--non-interactive", "--provider", "digitalocean",
		"--region", "sgp1", "--size", "s-1vcpu-1gb", "--no-storage", "--spot")
	if err == nil {
		t.Fatal("expected explicit --spot on digitalocean to fail, got success")
	}
	if !strings.Contains(errOut, "spot is not supported on digitalocean") {
		t.Errorf("expected spot-unsupported error, got: %q", errOut)
	}
}

func TestInitGeneratesNtfyTopic(t *testing.T) {
	cfgDir := t.TempDir()

	if _, errOut, err := initDO(t, cfgDir, "demo"); err != nil {
		t.Fatalf("init failed: %v\nstderr: %s", err, errOut)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "projects", "demo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "topic: serverku-demo-") {
		t.Errorf("config missing generated ntfy topic:\n%s", data)
	}
}

func TestNtfyShowsSubscribeInstructions(t *testing.T) {
	cfgDir := t.TempDir()

	if _, errOut, err := initDO(t, cfgDir, "demo"); err != nil {
		t.Fatalf("init failed: %v\nstderr: %s", err, errOut)
	}

	out, _, err := runCLI(t, "--config-dir", cfgDir, "ntfy", "demo")
	if err != nil {
		t.Fatalf("ntfy command failed: %v", err)
	}
	for _, want := range []string{
		"Topic:  serverku-demo-",
		"https://ntfy.sh",
		"iPhone",
		"Android",
		"--test",
		"heartbeat_hours: 6",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ntfy output missing %q:\n%s", want, out)
		}
	}
}

func TestNotifyTestSendsToConfiguredChannels(t *testing.T) {
	cfgDir := t.TempDir()

	if _, errOut, err := initDO(t, cfgDir, "demo"); err != nil {
		t.Fatalf("init failed: %v\nstderr: %s", err, errOut)
	}

	// Point the project's ntfy server at a local fake and capture publishes.
	var got struct {
		path  string
		title string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.title = r.Header.Get("Title")
	}))
	t.Cleanup(srv.Close)

	cfgPath := filepath.Join(cfgDir, "projects", "demo.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(data), "    ntfy:\n", "    ntfy:\n        server: "+srv.URL+"\n", 1)
	if err := os.WriteFile(cfgPath, []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runCLI(t, "--config-dir", cfgDir, "notify", "test", "demo")
	if err != nil {
		t.Fatalf("notify test failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "✓ ntfy") || !strings.Contains(out, "All channels OK") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.HasPrefix(got.path, "/serverku-demo-") {
		t.Errorf("fake server got path %q, want the project topic", got.path)
	}
	if got.title == "" {
		t.Error("expected a Title header on the test publish")
	}
}

func TestNotifyTestNoChannelsFails(t *testing.T) {
	cfgDir := t.TempDir()

	if _, errOut, err := initDO(t, cfgDir, "demo"); err != nil {
		t.Fatalf("init failed: %v\nstderr: %s", err, errOut)
	}

	// Strip the generated topic so no channel is configured.
	cfgPath := filepath.Join(cfgDir, "projects", "demo.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var stripped []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "topic:") || strings.Contains(line, "ntfy:") {
			continue
		}
		stripped = append(stripped, line)
	}
	if err := os.WriteFile(cfgPath, []byte(strings.Join(stripped, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	_, errOut, err := runCLI(t, "--config-dir", cfgDir, "notify", "test", "demo")
	if err == nil {
		t.Fatal("expected error when no channels configured")
	}
	if !strings.Contains(errOut, "notify setup") {
		t.Errorf("error should point at notify setup: %q", errOut)
	}
}

func TestNtfyUnknownProjectFails(t *testing.T) {
	cfgDir := t.TempDir()

	_, _, err := runCLI(t, "--config-dir", cfgDir, "ntfy", "nope")
	if err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestInitGCPRequiresProjectIDAndZone(t *testing.T) {
	cfgDir := t.TempDir()

	_, errOut, err := runCLI(t, "--config-dir", cfgDir, "init", "demo",
		"--non-interactive", "--provider", "gcp", "--region", "us-central1", "--no-storage")
	if err == nil {
		t.Fatal("expected GCP init without project_id/zone to fail, got success")
	}
	if !strings.Contains(errOut, "project_id is required") || !strings.Contains(errOut, "zone is required") {
		t.Errorf("expected GCP required-field errors, got: %q", errOut)
	}
}

func TestListEmpty(t *testing.T) {
	cfgDir := t.TempDir()

	out, _, err := runCLI(t, "--config-dir", cfgDir, "list")
	if err != nil {
		t.Fatalf("list on empty dir failed: %v", err)
	}
	if !strings.Contains(out, "No projects found") {
		t.Errorf("expected 'No projects found', got: %q", out)
	}
}

func TestBackupWithoutStorageFails(t *testing.T) {
	cfgDir := t.TempDir()

	// initDO creates a --no-storage project; backup must fail before any cloud call.
	if _, errOut, err := initDO(t, cfgDir, "demo"); err != nil {
		t.Fatalf("init failed: %v\nstderr: %s", err, errOut)
	}

	_, errOut, err := runCLI(t, "--config-dir", cfgDir, "backup", "demo")
	if err == nil {
		t.Fatal("expected backup to fail for a project without persistent storage")
	}
	if !strings.Contains(errOut, "no persistent storage") {
		t.Errorf("expected 'no persistent storage' error, got: %q", errOut)
	}
}

func TestFormatVersion(t *testing.T) {
	cases := []struct {
		name    string
		version string
		commit  string
		date    string
		dirty   bool
		want    string
	}{
		{"release tag only", "v0.1.0", "", "", false, "v0.1.0"},
		{"release with commit and date", "v0.1.0", "abcdef1234567890", "2026-07-15T10:00:00Z", false, "v0.1.0 (abcdef123456, 2026-07-15T10:00:00Z)"},
		{"dev clean with commit", "dev", "abcdef1234567890", "", false, "dev (abcdef123456)"},
		{"dev dirty with commit", "dev", "abcdef1234567890", "", true, "dev (abcdef123456-dirty)"},
		{"dirty without commit", "dev", "", "", true, "dev (dirty)"},
		{"short commit not truncated", "dev", "abc123", "", false, "dev (abc123)"},
		{"date only", "dev", "", "2026-07-15T10:00:00Z", false, "dev (2026-07-15T10:00:00Z)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatVersion(tc.version, tc.commit, tc.date, tc.dirty); got != tc.want {
				t.Errorf("formatVersion(%q,%q,%q,%v) = %q, want %q", tc.version, tc.commit, tc.date, tc.dirty, got, tc.want)
			}
		})
	}
}
