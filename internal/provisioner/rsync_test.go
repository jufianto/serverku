package provisioner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRsyncPassesCustomKeyPathAsOneArgument(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is not installed")
	}
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "ssh-args")
	fakeSSH := filepath.Join(dir, "ssh")
	if err := os.WriteFile(fakeSSH, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SERVERKU_TEST_SSH_ARGS\"\nexit 23\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SERVERKU_TEST_SSH_ARGS", argsPath)
	keyPath := filepath.Join(dir, "keys with 'quotes' $()", "id_ed25519")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rsyncDir(ctx, t.TempDir(), keyPath, "serverku", "fake.invalid", "/data"); err == nil {
		t.Fatal("fake SSH should stop before any transfer")
	}
	data, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(data)), "\n")
	found := false
	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) {
			found = args[i+1] == keyPath
		}
	}
	if !found {
		t.Fatalf("custom path was split or altered: %q", args)
	}
}
