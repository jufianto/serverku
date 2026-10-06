package config

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestProjectSSHKeysAreIndependentAndOpenSSHCompatible(t *testing.T) {
	s := newTestStore(t)
	a, err := s.ResolveProjectSSHKey(&ProjectConfig{Name: "alpha"}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.ResolveProjectSSHKey(&ProjectConfig{Name: "beta"}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if a.PublicKey == b.PublicKey || a.PrivatePath == b.PrivatePath || !a.Managed || !b.Managed {
		t.Fatal("projects must have independent managed keys")
	}
	for _, key := range []*ProjectSSHKey{a, b} {
		info, err := os.Stat(key.PrivatePath)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("private key permissions: %v, %v", info, err)
		}
		if bin, err := exec.LookPath("ssh-keygen"); err == nil {
			public, err := exec.Command(bin, "-y", "-f", key.PrivatePath).Output()
			if err != nil {
				t.Fatalf("OpenSSH cannot read generated key: %v", err)
			}
			if !bytes.HasPrefix(public, bytes.TrimSpace([]byte(key.PublicKey))) {
				t.Fatal("OpenSSH and Go disagree on key identity")
			}
		}
	}
	aAgain, err := s.ResolveProjectSSHKey(&ProjectConfig{Name: "alpha"}, nil, true)
	if err != nil || aAgain.PublicKey != a.PublicKey {
		t.Fatal("repeated setup rotated a project key")
	}
	if _, err := os.Stat(filepath.Join(s.KeysDir(), keyFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("new setup should not create a shared key")
	}
}

func TestProjectSSHKeysCustomAndLegacy(t *testing.T) {
	s := newTestStore(t)
	legacyPath, legacyPublic, err := s.EnsureSSHKeys()
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := s.ResolveProjectSSHKey(&ProjectConfig{Name: "old"}, &ProjectState{VMName: "serverku-old"}, true)
	if err != nil || legacy.PrivatePath != legacyPath || legacy.Managed || legacy.PublicKey != legacyPublic {
		t.Fatalf("tracked legacy VM changed identity: %+v, %v", legacy, err)
	}
	custom, err := s.ResolveProjectSSHKey(&ProjectConfig{Name: "custom", SSH: SSHConfig{PrivateKey: "../keys/serverku_rsa"}}, nil, true)
	if err != nil || custom.PrivatePath != legacyPath || custom.Managed {
		t.Fatalf("custom resolution: %+v, %v", custom, err)
	}
	if _, err := s.ResolveProjectSSHKey(&ProjectConfig{Name: "bad", SSH: SSHConfig{PrivateKey: "/missing-custom-key"}}, nil, true); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing custom key should fail: %v", err)
	}
	if _, err := s.ResolveProjectSSHKey(&ProjectConfig{Name: "uninitialized"}, nil, false); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only resolution should not generate: %v", err)
	}
	if _, err := s.ResolveProjectSSHKey(&ProjectConfig{Name: "../escape"}, nil, true); err == nil {
		t.Fatal("unsafe project name accepted")
	}
}

func TestRecordedProjectKeyNeverRotatesWhenMissing(t *testing.T) {
	s := newTestStore(t)
	cfg := &ProjectConfig{Name: "alpha"}
	key, err := s.ResolveProjectSSHKey(cfg, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	state := &ProjectState{SSHPrivateKeyPath: key.PrivatePath, SSHPublicKey: key.PublicKey, SSHKeyManaged: true}
	if err := os.Remove(key.PrivatePath); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveProjectSSHKey(cfg, state, true); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recorded missing identity should fail: %v", err)
	}
}

func TestProjectSSHKeyConcurrentGeneration(t *testing.T) {
	s := newTestStore(t)
	var wg sync.WaitGroup
	keys := make(chan *ProjectSSHKey, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key, err := s.ResolveProjectSSHKey(&ProjectConfig{Name: "alpha"}, nil, true)
			keys <- key
			errs <- err
		}()
	}
	wg.Wait()
	close(keys)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var public string
	for key := range keys {
		if public != "" && public != key.PublicKey {
			t.Fatal("concurrent setup returned multiple key identities")
		}
		public = key.PublicKey
	}
}

func TestProjectSSHKeyReferencesAndEditingGuard(t *testing.T) {
	s := newTestStore(t)
	owner := &ProjectConfig{Name: "owner", Provider: "digitalocean", Region: "sgp1", VM: VMConfig{Size: "s-1vcpu-1gb"}}
	key, err := s.ResolveProjectSSHKey(owner, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	other := *owner
	other.Name = "other"
	other.SSH.PrivateKey = key.PrivatePath
	if err := s.SaveProject(&other); err != nil {
		t.Fatal(err)
	}
	used, err := s.ProjectSSHKeyInUse(owner.Name, key.PublicKey)
	if err != nil || !used {
		t.Fatalf("custom key references not found: %v, %v", used, err)
	}
	state := &ProjectState{SSHPrivateKeyPath: key.PrivatePath}
	if err := ValidateProjectChange(owner, &other, state); err == nil {
		t.Fatal("changing tracked key should be blocked")
	}
}
