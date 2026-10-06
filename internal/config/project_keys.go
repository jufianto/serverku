package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// ProjectSSHKey describes the key actually used for this project's VM.
type ProjectSSHKey struct {
	PrivatePath string
	PublicKey   string
	Managed     bool
}

// ResolveProjectSSHKey preserves a tracked VM's identity, including legacy
// shared keys. New lifecycles use a project key or the configured custom key.
// Reads never generate keys unless create is true.
func (s *Store) ResolveProjectSSHKey(cfg *ProjectConfig, state *ProjectState, create bool) (*ProjectSSHKey, error) {
	if !isValidName(cfg.Name) {
		return nil, fmt.Errorf("invalid project name %q", cfg.Name)
	}
	if state != nil && state.SSHPrivateKeyPath != "" {
		key, err := readProjectSSHKey(state.SSHPrivateKeyPath, state.SSHKeyManaged)
		if err != nil {
			return nil, err
		}
		if state.SSHPublicKey != "" && key.PublicKey != state.SSHPublicKey {
			return nil, fmt.Errorf("SSH private key no longer matches the key recorded for project %q", cfg.Name)
		}
		return key, nil
	}
	if state != nil && (state.VMID != "" || state.VMName != "") {
		return readProjectSSHKey(filepath.Join(s.KeysDir(), keyFileName), false)
	}
	if cfg.SSH.PrivateKey != "" {
		path := cfg.SSH.PrivateKey
		if strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			path = filepath.Join(home, path[2:])
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(s.ProjectsDir(), path)
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		return readProjectSSHKey(absolute, false)
	}
	path, err := filepath.Abs(filepath.Join(s.KeysDir(), cfg.Name, "id_ed25519"))
	if err != nil {
		return nil, err
	}
	key, err := readProjectSSHKey(path, true)
	if err == nil || !create || !errors.Is(err, os.ErrNotExist) {
		return key, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	encoded, err := ssh.MarshalPrivateKey(privateKey, "serverku-"+cfg.Name)
	if err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".key-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	if _, err := temp.Write(pem.EncodeToMemory(encoded)); err != nil {
		_ = temp.Close()
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, err
	}
	// Linking publishes a complete private key without replacing a concurrently
	// generated key (or overwriting a partially existing identity).
	if err := os.Link(temp.Name(), path); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	key, err = readProjectSSHKey(path, true)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path+".pub", []byte(key.PublicKey), 0600); err != nil {
		return nil, err
	}
	return key, nil
}

func readProjectSSHKey(path string, managed bool) (*ProjectSSHKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read SSH private key %q: %w", path, err)
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse SSH private key %q (an unencrypted key is required): %w", path, err)
	}
	return &ProjectSSHKey{PrivatePath: path, PublicKey: string(ssh.MarshalAuthorizedKey(signer.PublicKey())), Managed: managed}, nil
}

// ProjectSSHKeyInUse checks other project definitions and tracked identities
// before removing a generated key that might also have been supplied as custom.
func (s *Store) ProjectSSHKeyInUse(name, publicKey string) (bool, error) {
	wanted, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return false, err
	}
	names, err := s.ListProjects()
	if err != nil {
		return false, err
	}
	for _, other := range names {
		if other == name {
			continue
		}
		cfg, err := s.LoadProject(other)
		if err != nil {
			return false, err
		}
		state, err := s.LoadState(other)
		if err != nil {
			return false, err
		}
		pub := state.SSHPublicKey
		if pub == "" {
			key, err := s.ResolveProjectSSHKey(cfg, state, false)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return false, err
			}
			pub = key.PublicKey
		}
		parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pub))
		if err != nil {
			return false, err
		}
		if ssh.FingerprintSHA256(parsed) == ssh.FingerprintSHA256(wanted) {
			return true, nil
		}
	}
	return false, nil
}

// DeleteProjectSSHKey removes only the canonical generated key files, never
// a custom or legacy shared key. Callers first remove any owned cloud key.
func (s *Store) DeleteProjectSSHKey(name string) error {
	if !isValidName(name) {
		return fmt.Errorf("invalid project name %q", name)
	}
	dir := filepath.Join(s.KeysDir(), name)
	for _, filename := range []string{"id_ed25519.pub", "id_ed25519"} {
		if err := os.Remove(filepath.Join(dir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
