package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

const (
	keyFileName = "serverku_rsa"
	keyFilePub  = "serverku_rsa.pub"
	keyBitSize  = 4096
)

// EnsureSSHKeys generates an SSH key pair if it doesn't already exist.
// Keys are stored in the store's keys directory.
// Returns the path to the private key and the public key string.
func (s *Store) EnsureSSHKeys() (privateKeyPath string, pubKeyString string, err error) {
	privPath := filepath.Join(s.KeysDir(), keyFileName)
	pubPath := filepath.Join(s.KeysDir(), keyFilePub)

	// If both files exist, read and return the public key
	if fileExists(privPath) && fileExists(pubPath) {
		pubData, err := os.ReadFile(pubPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read public key: %w", err)
		}
		return privPath, string(pubData), nil
	}

	// Generate new key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, keyBitSize)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Encode private key to PEM
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Write private key with restricted permissions
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		return "", "", fmt.Errorf("failed to write private key: %w", err)
	}

	// Generate SSH public key
	sshPub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate SSH public key: %w", err)
	}

	pubKeyStr := string(ssh.MarshalAuthorizedKey(sshPub))

	// Write public key
	if err := os.WriteFile(pubPath, []byte(pubKeyStr), 0644); err != nil {
		return "", "", fmt.Errorf("failed to write public key: %w", err)
	}

	return privPath, pubKeyStr, nil
}

// GetSSHPublicKey reads the SSH public key from the keys directory.
// Returns an error if the key doesn't exist.
func (s *Store) GetSSHPublicKey() (string, error) {
	pubPath := filepath.Join(s.KeysDir(), keyFilePub)
	data, err := os.ReadFile(pubPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("SSH keys not found, run 'serverku init' first")
		}
		return "", fmt.Errorf("failed to read SSH public key: %w", err)
	}
	return string(data), nil
}

// GetSSHPrivateKeyPath returns the path to the SSH private key.
// Returns an error if the key doesn't exist.
func (s *Store) GetSSHPrivateKeyPath() (string, error) {
	privPath := filepath.Join(s.KeysDir(), keyFileName)
	if !fileExists(privPath) {
		return "", fmt.Errorf("SSH private key not found, run 'serverku init' first")
	}
	return privPath, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
