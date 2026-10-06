package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const editableProject = `# Keep my project comment
name: demo
provider: digitalocean
region: sgp1
vm:
  size: s-1vcpu-1gb
  image: ubuntu-24-04-x64
  spot: false
storage:
  enabled: false
  size_gb: 20
  mount_path: /data
compose_file: docker-compose.yml
sync_dir: ./app
notifications:
  ntfy:
    topic: original-topic
hooks:
  pre_up: [echo hello]
startup_commands: [echo ready]
x-custom: preserved
`

func editingFixture(t *testing.T) (string, string) {
	t.Helper()
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	dir := filepath.Join(t.TempDir(), "config with spaces")
	for _, sub := range []string{"projects", "state", "keys"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "projects", "demo.yaml")
	for file, data := range map[string]string{path: editableProject, filepath.Join(dir, "state", "demo.json"): `{"project_name":"demo","status":"stopped"}`, filepath.Join(dir, "keys", "serverku_rsa"): "key-must-not-change"} {
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, path
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func fakeEditor(t *testing.T, body string) {
	t.Helper()
	editor := filepath.Join(t.TempDir(), "fake editor")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\nset -e\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", `"`+editor+`" --wait`)
	// VISUAL must take precedence over EDITOR.
	t.Setenv("EDITOR", "nonexistent-fallback-editor")
}

func TestReinitPreservesOtherSettingsAndRuntimeFiles(t *testing.T) {
	dir, path := editingFixture(t)
	statePath, keyPath := filepath.Join(dir, "state", "demo.json"), filepath.Join(dir, "keys", "serverku_rsa")
	originalState, originalKey := readTestFile(t, statePath), readTestFile(t, keyPath)
	out, errOut, err := runCLI(t, "--config-dir", dir, "reinit", "demo", "--non-interactive", "--size", "s-2vcpu-2gb", "--no-storage=false", "--storage-gb", "10")
	if err != nil {
		t.Fatalf("reinit failed: %v\n%s", err, errOut)
	}
	changed := readTestFile(t, path)
	for _, want := range []string{"# Keep my project comment", "size: s-2vcpu-2gb", "image: ubuntu-24-04-x64", "enabled: true", "size_gb: 10", "sync_dir: ./app", "original-topic", "pre_up:", "startup_commands:", "x-custom: preserved"} {
		if !strings.Contains(changed, want) {
			t.Errorf("lost %q:\n%s", want, changed)
		}
	}
	if readTestFile(t, statePath) != originalState || readTestFile(t, keyPath) != originalKey {
		t.Fatal("runtime state or key changed")
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "backups", "demo-*.yaml"))
	if len(backups) != 1 || readTestFile(t, backups[0]) != editableProject {
		t.Fatal("original backup missing or changed")
	}
	if !strings.Contains(out, "storage.enabled") || !strings.Contains(out, "Backup:") {
		t.Errorf("missing change summary: %s", out)
	}
}

func TestReinitNoChangesDoesNotRewrite(t *testing.T) {
	dir, path := editingFixture(t)
	out, errOut, err := runCLI(t, "--config-dir", dir, "reinit", "demo", "--non-interactive")
	if err != nil {
		t.Fatalf("reinit: %v\n%s", err, errOut)
	}
	if readTestFile(t, path) != editableProject || !strings.Contains(out, "No changes.") {
		t.Fatal("no-op reinit rewrote config")
	}
	if _, err := os.Stat(filepath.Join(dir, "backups")); !os.IsNotExist(err) {
		t.Fatal("no-op created backup")
	}
}

func TestReinitBlocksTrackedResources(t *testing.T) {
	for _, tc := range []struct {
		name, state string
		flags       []string
		want        string
	}{
		{"running", `{"status":"running","vm_name":"serverku-demo"}`, []string{"--size", "s-2vcpu-2gb"}, "no persistent disk"},
		{"placement", `{"status":"stopped","disk_name":"disk"}`, []string{"--region", "nyc3"}, "cannot change provider"},
		{"disk size", `{"status":"stopped","disk_name":"disk"}`, []string{"--storage-gb", "40"}, "cannot disable or resize"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, path := editingFixture(t)
			if err := os.WriteFile(filepath.Join(dir, "state", "demo.json"), []byte(tc.state), 0600); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"--config-dir", dir, "reinit", "demo", "--non-interactive"}, tc.flags...)
			_, errOut, err := runCLI(t, args...)
			if err == nil || !strings.Contains(errOut, tc.want) {
				t.Fatalf("unexpected result: %v\n%s", err, errOut)
			}
			if readTestFile(t, path) != editableProject {
				t.Fatal("rejected reinit overwrote config")
			}
		})
	}
}

func TestEditValidatesAndBacksUp(t *testing.T) {
	dir, path := editingFixture(t)
	replacement := filepath.Join(t.TempDir(), "replacement.yaml")
	edited := strings.Replace(editableProject, "s-1vcpu-1gb", "s-2vcpu-2gb", 1)
	if err := os.WriteFile(replacement, []byte(edited), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDIT_CONTENT", replacement)
	fakeEditor(t, "[ \"$1\" = --wait ]\ncp \"$EDIT_CONTENT\" \"$2\"\n")
	out, errOut, err := runCLI(t, "--config-dir", dir, "edit", "demo")
	if err != nil {
		t.Fatalf("edit failed: %v\n%s", err, errOut)
	}
	if readTestFile(t, path) != edited {
		t.Fatal("saved YAML text differs from editor output")
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "backups", "demo-*.yaml"))
	if len(backups) != 1 || readTestFile(t, backups[0]) != editableProject {
		t.Fatal("missing original backup")
	}
	info, err := os.Stat(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("backup permissions = %o", info.Mode().Perm())
	}
	if !strings.Contains(out, "Backup:") {
		t.Errorf("missing backup path: %s", out)
	}
	names, _ := filepath.Glob(filepath.Join(dir, "projects", ".demo-edit-*", "demo.yaml"))
	if len(names) != 0 {
		t.Fatal("successful edit left temporary files")
	}
}

func TestEditFailuresKeepOriginalAndRecoverEdits(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"invalid YAML", "printf 'name: [\\n' > \"$2\"\n", "invalid project YAML"},
		{"name changed", "sed 's/name: demo/name: other/' \"$2\" > \"$2.next\"\nmv \"$2.next\" \"$2\"\n", "name must remain"},
		{"editor failure", "exit 4\n", "editor failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, path := editingFixture(t)
			fakeEditor(t, tc.body)
			_, errOut, err := runCLI(t, "--config-dir", dir, "edit", "demo")
			if err == nil || !strings.Contains(errOut, tc.want) || !strings.Contains(errOut, "edits retained at") {
				t.Fatalf("unexpected error: %v\n%s", err, errOut)
			}
			if readTestFile(t, path) != editableProject {
				t.Fatal("failed edit changed original")
			}
			files, _ := filepath.Glob(filepath.Join(dir, "projects", ".demo-edit-*", "demo.yaml"))
			if len(files) != 1 {
				t.Fatal("recovery file missing")
			}
			info, err := os.Stat(files[0])
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0600 {
				t.Fatal("recovery file not private")
			}
			out, _, err := runCLI(t, "--config-dir", dir, "list")
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out, ".demo-edit-") {
				t.Fatal("recovery file listed as a project")
			}
		})
	}
}

func TestEditCanRepairMalformedProject(t *testing.T) {
	dir, path := editingFixture(t)
	if err := os.WriteFile(path, []byte("name: [\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(t.TempDir(), "fixed.yaml")
	if err := os.WriteFile(fixture, []byte(editableProject), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDIT_CONTENT", fixture)
	fakeEditor(t, "cp \"$EDIT_CONTENT\" \"$2\"\n")
	_, errOut, err := runCLI(t, "--config-dir", dir, "edit", "demo")
	if err != nil {
		t.Fatalf("repair failed: %v\n%s", err, errOut)
	}
	if readTestFile(t, path) != editableProject {
		t.Fatal("repair not saved")
	}
}

func TestEditNoChangesAndMissingProject(t *testing.T) {
	dir, path := editingFixture(t)
	fakeEditor(t, "exit 0\n")
	out, errOut, err := runCLI(t, "--config-dir", dir, "edit", "demo")
	if err != nil {
		t.Fatalf("edit: %v\n%s", err, errOut)
	}
	if !strings.Contains(out, "No changes.") || readTestFile(t, path) != editableProject {
		t.Fatal("no-op edit changed config")
	}
	for _, command := range []string{"edit", "reinit"} {
		_, errOut, err := runCLI(t, "--config-dir", dir, command, "missing")
		if err == nil || !strings.Contains(errOut, "not found") {
			t.Fatalf("missing project: %v\n%s", err, errOut)
		}
	}
}

func TestEditGuardsPlacementWhileVMTracked(t *testing.T) {
	dir, path := editingFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "state", "demo.json"), []byte(`{"status":"running","vm_name":"serverku-demo"}`), 0600); err != nil {
		t.Fatal(err)
	}
	fakeEditor(t, "sed 's/region: sgp1/region: nyc3/' \"$2\" > \"$2.next\"\nmv \"$2.next\" \"$2\"\n")
	_, errOut, err := runCLI(t, "--config-dir", dir, "edit", "demo")
	if err == nil || !strings.Contains(errOut, "cannot change provider") {
		t.Fatalf("placement change: %v\n%s", err, errOut)
	}
	if readTestFile(t, path) != editableProject {
		t.Fatal("rejected placement saved")
	}
}

func TestEditRepairsInvalidSettingsWithTrackedVM(t *testing.T) {
	dir, path := editingFixture(t)
	broken := strings.Replace(editableProject, "spot: false", "spot: true", 1)
	if err := os.WriteFile(path, []byte(broken), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state", "demo.json"), []byte(`{"status":"running","vm_name":"serverku-demo"}`), 0600); err != nil {
		t.Fatal(err)
	}
	fakeEditor(t, "sed 's/spot: true/spot: false/' \"$2\" > \"$2.next\"\nmv \"$2.next\" \"$2\"\n")
	_, errOut, err := runCLI(t, "--config-dir", dir, "edit", "demo")
	if err != nil {
		t.Fatalf("repair rejected: %v\n%s", err, errOut)
	}
	if readTestFile(t, path) != editableProject {
		t.Fatal("repair not saved")
	}
}
