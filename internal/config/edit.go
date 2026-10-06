package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProjectPath resolves only valid project names, preventing directory traversal.
func (s *Store) ProjectPath(name string) (string, error) {
	if !isValidName(name) {
		return "", fmt.Errorf("invalid project name %q: use lowercase letters, digits and hyphens", name)
	}
	return filepath.Abs(filepath.Join(s.ProjectsDir(), name+".yaml"))
}

// ParseProjectDocument accepts exactly one YAML project document.
func ParseProjectDocument(name string, data []byte) (*ProjectConfig, error) {
	var cfg ProjectConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid project YAML: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("project YAML must contain exactly one document")
	}
	if cfg.Name != name {
		return nil, fmt.Errorf("project name must remain %q (got %q)", name, cfg.Name)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ValidateProjectChange keeps existing resources reachable through their config.
func ValidateProjectChange(before, after *ProjectConfig, state *ProjectState) error {
	hasVM := state.VMID != "" || state.VMName != "" || state.IsRunning() || state.Status == StatusStopping
	hasDisk := state.DiskID != "" || state.DiskName != ""
	if before != nil && before.SSH.PrivateKey != after.SSH.PrivateKey && (hasVM || state.SSHPrivateKeyPath != "" || state.SSHKeyOwned) {
		return fmt.Errorf("cannot change the SSH key while a VM or SSH identity is tracked; destroy the project resources before changing ssh.private_key (persistent data must be backed up first)")
	}
	if state.SSHKeyOwned && before != nil && before.Provider != after.Provider {
		return fmt.Errorf("cannot change provider while an owned SSH key is tracked; destroy its resources first")
	}
	if !hasVM && !hasDisk {
		return nil
	}
	if before == nil {
		return fmt.Errorf("cannot verify resource placement against the malformed original config; restore a valid config from backup before changing tracked resources")
	}
	projectChanged := (before.Provider == "gcp" || after.Provider == "gcp") && before.ProjectID != after.ProjectID
	if before.Provider != after.Provider || projectChanged || before.Region != after.Region || before.Zone != after.Zone {
		return fmt.Errorf("cannot change provider, project ID, region or zone while a VM or persistent disk is tracked; keep the current placement or migrate resources first")
	}
	if hasDisk && (before.Storage.Enabled != after.Storage.Enabled || before.Storage.SizeGB != after.Storage.SizeGB) {
		return fmt.Errorf("cannot disable or resize a tracked persistent disk through config editing; use a separate storage migration")
	}
	return nil
}

// SaveProjectDocument replaces a checked document and saves the previous bytes
// in a private backup. expected detects changes made while an editor was open.
func (s *Store) SaveProjectDocument(name string, expected, data []byte) (string, error) {
	path, err := s.ProjectPath(name)
	if err != nil {
		return "", err
	}
	if bytes.Equal(expected, data) {
		return "", nil
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(current, expected) {
		return "", fmt.Errorf("project changed since it was opened; refusing to overwrite newer changes")
	}
	backupDir := filepath.Join(s.baseDir, "backups")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return "", err
	}
	backup, err := os.CreateTemp(backupDir, name+"-*.yaml")
	if err != nil {
		return "", err
	}
	backupPath := backup.Name()
	if _, err := backup.Write(expected); err != nil {
		_ = backup.Close()
		return "", err
	}
	if err := backup.Close(); err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(s.ProjectsDir(), "."+name+"-save-*.yaml")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return "", err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	current, err = os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(current, expected) {
		return "", fmt.Errorf("project changed during save; refusing to overwrite newer changes")
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return "", err
	}
	return backupPath, nil
}

// MergeProjectSettings patches only changed wizard fields, retaining comments
// and unrecognized settings in the original document.
func MergeProjectSettings(data []byte, before, after *ProjectConfig) ([]byte, []string, error) {
	var original, oldNode, newNode yaml.Node
	if err := yaml.Unmarshal(data, &original); err != nil {
		return nil, nil, err
	}
	if err := oldNode.Encode(before); err != nil {
		return nil, nil, err
	}
	if err := newNode.Encode(after); err != nil {
		return nil, nil, err
	}
	paths := []string{"provider", "project_id", "region", "zone", "vm.size", "vm.image", "vm.spot", "storage.enabled", "storage.size_gb", "storage.mount_path", "compose_file", "ssh.private_key"}
	var changes []string
	for _, path := range paths {
		parts := strings.Split(path, ".")
		oldValue, newValue := nodeAt(&oldNode, parts), nodeAt(&newNode, parts)
		var oldData, newData any
		if oldValue != nil {
			if err := oldValue.Decode(&oldData); err != nil {
				return nil, nil, err
			}
		}
		if newValue != nil {
			if err := newValue.Decode(&newData); err != nil {
				return nil, nil, err
			}
		}
		if reflect.DeepEqual(oldData, newData) {
			continue
		}
		changes = append(changes, fmt.Sprintf("%s: %v → %v", path, oldData, newData))
		if err := setNodeAt(original.Content[0], parts, newValue); err != nil {
			return nil, nil, err
		}
	}
	if len(changes) == 0 {
		return data, nil, nil
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&original); err != nil {
		return nil, nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, nil, err
	}
	return output.Bytes(), changes, nil
}

func nodeAt(node *yaml.Node, path []string) *yaml.Node {
	for _, key := range path {
		if node == nil {
			return nil
		}
		if node.Kind == yaml.DocumentNode {
			node = node.Content[0]
		}
		var child *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				child = node.Content[i+1]
				break
			}
		}
		node = child
	}
	return node
}

func setNodeAt(node *yaml.Node, path []string, value *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("cannot update %s: expected a YAML mapping", strings.Join(path, "."))
	}
	key := path[0]
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value != key {
			continue
		}
		if len(path) > 1 {
			return setNodeAt(node.Content[i+1], path[1:], value)
		}
		if value == nil {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return nil
		}
		replacement := *value
		replacement.HeadComment = node.Content[i+1].HeadComment
		replacement.LineComment = node.Content[i+1].LineComment
		replacement.FootComment = node.Content[i+1].FootComment
		node.Content[i+1] = &replacement
		return nil
	}
	if value == nil {
		return nil
	}
	child := value
	if len(path) > 1 {
		child = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		if err := setNodeAt(child, path[1:], value); err != nil {
			return err
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, child)
	return nil
}
