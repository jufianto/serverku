package provisioner

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Script generation tests (6.1)
// ---------------------------------------------------------------------------

func TestMountDiskScript(t *testing.T) {
	script := mountDiskScript("myproject-data", "/data")

	// Should reference the stable by-id device path
	if !strings.Contains(script, "/dev/disk/by-id/google-myproject-data") {
		t.Error("expected stable device path /dev/disk/by-id/google-myproject-data")
	}

	// Should mount to the given path
	if !strings.Contains(script, "/data") {
		t.Error("expected mount path /data in script")
	}

	// Must not blindly format -- blkid guard must be present
	if !strings.Contains(script, "blkid") {
		t.Error("expected blkid check before mkfs")
	}

	// mkfs should be present (for the new-disk case)
	if !strings.Contains(script, "mkfs.ext4") {
		t.Error("expected mkfs.ext4 in script")
	}

	// fstab entry must use nofail so a missing disk doesn't brick the VM
	if !strings.Contains(script, "nofail") {
		t.Error("expected nofail fstab option")
	}

	// Ownership should be transferred to the serverku user
	if !strings.Contains(script, "chown serverku") {
		t.Error("expected chown serverku in script")
	}
}

func TestMountDiskScriptDifferentDisk(t *testing.T) {
	script := mountDiskScript("other-disk", "/mnt/storage")

	if !strings.Contains(script, "/dev/disk/by-id/google-other-disk") {
		t.Errorf("expected device path for 'other-disk', got script:\n%s", script)
	}
	if !strings.Contains(script, "/mnt/storage") {
		t.Errorf("expected mount path /mnt/storage, got script:\n%s", script)
	}
}

func TestWriteComposeScript(t *testing.T) {
	content := "version: '3'\nservices:\n  web:\n    image: nginx\n"
	script := writeComposeScript(content, "/data")

	// Must write to the expected file path
	if !strings.Contains(script, "/data/docker-compose.yml") {
		t.Error("expected target path /data/docker-compose.yml")
	}

	// Must use the safe heredoc delimiter
	if !strings.Contains(script, "SERVERKU_COMPOSE_EOF") {
		t.Error("expected SERVERKU_COMPOSE_EOF heredoc delimiter")
	}

	// The compose content itself must be embedded
	if !strings.Contains(script, "image: nginx") {
		t.Error("expected compose content to be embedded in script")
	}
}

func TestWriteComposeScriptCreatesDir(t *testing.T) {
	script := writeComposeScript("x: y\n", "/home/serverku")

	if !strings.Contains(script, "mkdir") {
		t.Error("expected mkdir to create target directory")
	}
}

func TestComposeUpScript(t *testing.T) {
	script := composeUpScript("/data")

	if !strings.Contains(script, "cd \"/data\"") {
		t.Errorf("expected cd to /data, got: %s", script)
	}
	if !strings.Contains(script, "docker compose up -d") {
		t.Error("expected docker compose up -d")
	}
}

func TestTeardownScriptBothEnabled(t *testing.T) {
	script := teardownScript("/data", "/data", true, true)

	if !strings.Contains(script, "docker compose down") {
		t.Error("expected docker compose down when hasCompose=true")
	}
	if !strings.Contains(script, "umount") {
		t.Error("expected umount when hasStorage=true")
	}
	// Must be best-effort
	if !strings.Contains(script, "set +e") {
		t.Error("expected set +e for best-effort teardown")
	}
}

func TestTeardownScriptNoCompose(t *testing.T) {
	script := teardownScript("", "/data", false, true)

	if strings.Contains(script, "docker compose down") {
		t.Error("should NOT call docker compose down when hasCompose=false")
	}
	if !strings.Contains(script, "umount") {
		t.Error("expected umount when hasStorage=true")
	}
}

func TestTeardownScriptNoStorage(t *testing.T) {
	script := teardownScript("/home/serverku", "", true, false)

	if !strings.Contains(script, "docker compose down") {
		t.Error("expected docker compose down when hasCompose=true")
	}
	if strings.Contains(script, "umount") {
		t.Error("should NOT call umount when hasStorage=false")
	}
}

func TestTeardownScriptNeitherEnabled(t *testing.T) {
	script := teardownScript("", "", false, false)

	if strings.Contains(script, "docker compose") {
		t.Error("should not reference docker compose when hasCompose=false")
	}
	if strings.Contains(script, "umount") {
		t.Error("should not reference umount when hasStorage=false")
	}
}

// ---------------------------------------------------------------------------
// NoopProvisioner tests (6.2)
// ---------------------------------------------------------------------------

func TestNoopProvisionerProvision(t *testing.T) {
	np := &NoopProvisioner{}
	err := np.Provision(context.Background(), ProvisionOpts{
		Host:            "1.2.3.4",
		PrivateKeyPath:  "/some/key",
		SSHUser:         "serverku",
		StorageEnabled:  true,
		DiskName:        "my-disk",
		MountPath:       "/data",
		ComposeContent:  "version: '3'\n",
		StartupCommands: []string{"echo hello"},
	})
	if err != nil {
		t.Errorf("NoopProvisioner.Provision() returned error: %v", err)
	}
}

func TestNoopProvisionerTeardown(t *testing.T) {
	np := &NoopProvisioner{}
	err := np.Teardown(context.Background(), TeardownOpts{
		Host:           "1.2.3.4",
		PrivateKeyPath: "/some/key",
		SSHUser:        "serverku",
		StorageEnabled: true,
		MountPath:      "/data",
		ComposeDir:     "/data",
	})
	if err != nil {
		t.Errorf("NoopProvisioner.Teardown() returned error: %v", err)
	}
}
