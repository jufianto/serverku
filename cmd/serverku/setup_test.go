package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jufianto/serverku/internal/config"
)

// withStore points the package-level store at a fresh throwaway store for the
// duration of a test.
func withStore(t *testing.T) *config.Store {
	t.Helper()
	s, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	prev := store
	store = s
	t.Cleanup(func() { store = prev })
	return s
}

func TestResolveDOToken_EnvWins(t *testing.T) {
	s := withStore(t)
	if err := s.SaveCredential("digitalocean", "from-file"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DIGITALOCEAN_TOKEN", "from-env")

	got, err := resolveDOToken()
	if err != nil {
		t.Fatalf("resolveDOToken: %v", err)
	}
	if got != "from-env" {
		t.Errorf("got %q, want from-env (env must take precedence over file)", got)
	}
}

func TestResolveDOToken_FileFallback(t *testing.T) {
	s := withStore(t)
	if err := s.SaveCredential("digitalocean", "from-file"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DIGITALOCEAN_TOKEN", "")

	got, err := resolveDOToken()
	if err != nil {
		t.Fatalf("resolveDOToken: %v", err)
	}
	if got != "from-file" {
		t.Errorf("got %q, want from-file", got)
	}
}

func TestResolveDOToken_NoneConfigured(t *testing.T) {
	withStore(t)
	t.Setenv("DIGITALOCEAN_TOKEN", "")

	if _, err := resolveDOToken(); err == nil {
		t.Error("expected an error when no token is configured anywhere")
	}
}

func TestADCLocation_EnvVarWins(t *testing.T) {
	f := filepath.Join(t.TempDir(), "key.json")
	if err := os.WriteFile(f, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", f)

	path, ok := adcLocation()
	if !ok || path != f {
		t.Errorf("adcLocation() = (%q, %v), want (%q, true)", path, ok, f)
	}
}

func TestADCLocation_EnvVarPointsAtMissingFile(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(t.TempDir(), "nope.json"))
	// Point the well-known path at an empty config dir so nothing is found.
	t.Setenv("CLOUDSDK_CONFIG", t.TempDir())

	if path, ok := adcLocation(); ok {
		t.Errorf("adcLocation() = (%q, true), want not found", path)
	}
}

func TestADCLocation_WellKnownPath(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	cfgDir := t.TempDir()
	t.Setenv("CLOUDSDK_CONFIG", cfgDir)

	want := filepath.Join(cfgDir, "application_default_credentials.json")
	if err := os.WriteFile(want, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	path, ok := adcLocation()
	if !ok {
		t.Fatalf("adcLocation() not found, want %q", want)
	}
	if filepath.Clean(path) != filepath.Clean(want) {
		t.Errorf("adcLocation() = %q, want %q", path, want)
	}
}

func TestWellKnownADCPath_HonorsCloudSDKConfig(t *testing.T) {
	t.Setenv("CLOUDSDK_CONFIG", "/custom/gcloud")
	want := "/custom/gcloud/application_default_credentials.json"
	if got := wellKnownADCPath(); got != want {
		t.Errorf("wellKnownADCPath() = %q, want %q", got, want)
	}
}
