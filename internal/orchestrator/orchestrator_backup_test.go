package orchestrator

import (
	"context"
	"testing"

	"github.com/jufianto/serverku/internal/config"
)

func TestBackup_SnapshotsDisk(t *testing.T) {
	orch, mock, factory := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})

	// Bring it up so a disk exists in state.
	if _, err := orch.Up(context.Background(), "test-project", factory); err != nil {
		t.Fatalf("Up: %v", err)
	}

	var gotDisk, gotSnap string
	mock.snapshotDiskFunc = func(ctx context.Context, diskName, snapshotName string) (string, error) {
		gotDisk, gotSnap = diskName, snapshotName
		return "snap-123", nil
	}

	res, err := orch.Backup(context.Background(), "test-project", "my-snapshot", factory)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if res.SnapshotID != "snap-123" || res.SnapshotName != "my-snapshot" {
		t.Errorf("unexpected result: %+v", res)
	}
	if gotSnap != "my-snapshot" {
		t.Errorf("snapshot name passed to provider = %q, want my-snapshot", gotSnap)
	}
	if gotDisk != "serverku-test-project-data" {
		t.Errorf("disk name passed to provider = %q", gotDisk)
	}
	if !containsStr(mock.calls, "SnapshotDisk") {
		t.Errorf("expected SnapshotDisk to be called, calls: %v", mock.calls)
	}
}

func TestBackup_NoStorageFails(t *testing.T) {
	orch, _, factory := testSetup(t, config.StorageConfig{Enabled: false})

	if _, err := orch.Backup(context.Background(), "test-project", "snap", factory); err == nil {
		t.Fatal("expected error when project has no persistent storage")
	}
}

func TestBackup_NoDiskYetFails(t *testing.T) {
	// Storage enabled but never brought up, so no disk exists in state.
	orch, _, factory := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})

	if _, err := orch.Backup(context.Background(), "test-project", "snap", factory); err == nil {
		t.Fatal("expected error when project has no disk yet")
	}
}
