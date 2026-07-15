package main

import (
	"os"
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

func TestResolveGCPCredentialEnv_UserEnvWins(t *testing.T) {
	withStore(t)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/my/own/key.json")

	resolveGCPCredentialEnv()

	if got := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); got != "/my/own/key.json" {
		t.Errorf("a user-set GOOGLE_APPLICATION_CREDENTIALS must be left untouched, got %q", got)
	}
}

func TestResolveGCPCredentialEnv_UsesIsolatedADC(t *testing.T) {
	s := withStore(t)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	if err := os.MkdirAll(s.GcloudDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.GcloudADCPath(), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	resolveGCPCredentialEnv()

	if got := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); got != s.GcloudADCPath() {
		t.Errorf("expected env pointed at isolated ADC %q, got %q", s.GcloudADCPath(), got)
	}
}

func TestResolveGCPCredentialEnv_NoIsolatedADC(t *testing.T) {
	withStore(t)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	resolveGCPCredentialEnv()

	if got := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); got != "" {
		t.Errorf("expected env left empty when no isolated ADC exists, got %q", got)
	}
}
