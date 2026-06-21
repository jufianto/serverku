package hooks

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_EmptyIsNoop(t *testing.T) {
	if err := Run(context.Background(), "pre_up", nil, Options{}); err != nil {
		t.Fatalf("empty command list should be a no-op, got: %v", err)
	}
}

func TestRun_ExecutesInOrder(t *testing.T) {
	var out bytes.Buffer
	cmds := []string{"echo first", "echo second", "echo third"}

	if err := Run(context.Background(), "pre_up", cmds, Options{Stdout: &out}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := out.String()
	if got != "first\nsecond\nthird\n" {
		t.Errorf("unexpected output order: %q", got)
	}
}

func TestRun_StopsOnFirstFailure(t *testing.T) {
	var out bytes.Buffer
	cmds := []string{"echo before", "false", "echo after"}

	err := Run(context.Background(), "pre_up", cmds, Options{Stdout: &out})
	if err == nil {
		t.Fatal("expected error when a command fails")
	}
	if !strings.Contains(err.Error(), "pre_up hook failed at command 2") {
		t.Errorf("error should identify the failing command, got: %v", err)
	}
	if strings.Contains(out.String(), "after") {
		t.Errorf("commands after the failure should not run, got: %q", out.String())
	}
}

func TestRun_UsesWorkDir(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer

	if err := Run(context.Background(), "pre_up", []string{"pwd"}, Options{WorkDir: dir, Stdout: &out}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// macOS resolves TempDir under /private; compare by base name to stay portable.
	got := strings.TrimSpace(out.String())
	if filepath.Base(got) != filepath.Base(dir) {
		t.Errorf("expected pwd under %q, got %q", dir, got)
	}
}

func TestRun_InjectsEnv(t *testing.T) {
	var out bytes.Buffer
	opts := Options{Env: []string{"SERVERKU_PROJECT=myapp"}, Stdout: &out}

	if err := Run(context.Background(), "pre_up", []string{"echo $SERVERKU_PROJECT"}, opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.TrimSpace(out.String()) != "myapp" {
		t.Errorf("expected injected env to be visible, got %q", out.String())
	}
}

func TestRun_SupportsShellFeatures(t *testing.T) {
	var out bytes.Buffer
	// Pipes and && must work since commands run through `sh -c`.
	cmds := []string{"echo hello | tr a-z A-Z && echo done"}

	if err := Run(context.Background(), "pre_up", cmds, Options{Stdout: &out}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.String() != "HELLO\ndone\n" {
		t.Errorf("unexpected shell output: %q", out.String())
	}
}
