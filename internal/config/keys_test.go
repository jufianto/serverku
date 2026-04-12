package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStore_EnsureSSHKeys_GeneratesNewKeys(t *testing.T) {
	s := newTestStore(t)

	privPath, pubKey, err := s.EnsureSSHKeys()
	if err != nil {
		t.Fatalf("EnsureSSHKeys failed: %v", err)
	}

	// Private key should exist
	if _, err := os.Stat(privPath); err != nil {
		t.Errorf("private key not found at %s: %v", privPath, err)
	}

	// Private key should have restricted permissions
	info, _ := os.Stat(privPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("private key permissions should be 0600, got %o", info.Mode().Perm())
	}

	// Public key should be non-empty and start with ssh-rsa
	if pubKey == "" {
		t.Error("public key should not be empty")
	}
	if !strings.HasPrefix(pubKey, "ssh-rsa ") {
		t.Errorf("public key should start with 'ssh-rsa ', got prefix: %q", pubKey[:20])
	}

	// Public key file should exist
	pubPath := filepath.Join(s.KeysDir(), "serverku_rsa.pub")
	if _, err := os.Stat(pubPath); err != nil {
		t.Errorf("public key file not found: %v", err)
	}
}

func TestStore_EnsureSSHKeys_ReusesExisting(t *testing.T) {
	s := newTestStore(t)

	// Generate first time
	_, pubKey1, err := s.EnsureSSHKeys()
	if err != nil {
		t.Fatalf("first EnsureSSHKeys failed: %v", err)
	}

	// Call again - should return same key
	_, pubKey2, err := s.EnsureSSHKeys()
	if err != nil {
		t.Fatalf("second EnsureSSHKeys failed: %v", err)
	}

	if pubKey1 != pubKey2 {
		t.Error("EnsureSSHKeys should return the same key on second call")
	}
}

func TestStore_GetSSHPublicKey_NotGenerated(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetSSHPublicKey()
	if err == nil {
		t.Error("expected error when keys not generated yet")
	}
}

func TestStore_GetSSHPublicKey_AfterGeneration(t *testing.T) {
	s := newTestStore(t)

	_, expectedPub, _ := s.EnsureSSHKeys()

	gotPub, err := s.GetSSHPublicKey()
	if err != nil {
		t.Fatalf("GetSSHPublicKey failed: %v", err)
	}

	if gotPub != expectedPub {
		t.Error("GetSSHPublicKey returned different key than EnsureSSHKeys")
	}
}

func TestStore_GetSSHPrivateKeyPath_NotGenerated(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetSSHPrivateKeyPath()
	if err == nil {
		t.Error("expected error when keys not generated yet")
	}
}

func TestStore_GetSSHPrivateKeyPath_AfterGeneration(t *testing.T) {
	s := newTestStore(t)

	expectedPath, _, _ := s.EnsureSSHKeys()

	gotPath, err := s.GetSSHPrivateKeyPath()
	if err != nil {
		t.Fatalf("GetSSHPrivateKeyPath failed: %v", err)
	}

	if gotPath != expectedPath {
		t.Errorf("path mismatch: got %q, want %q", gotPath, expectedPath)
	}
}
