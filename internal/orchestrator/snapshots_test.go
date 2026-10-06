package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/provider"
)

func TestDownPreservesBackupAndDestroyDeletesIt(t *testing.T) {
	o, m, factory := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})
	ctx := context.Background()
	if _, err := o.Up(ctx, "test-project", factory); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Backup(ctx, "test-project", "custom-backup-name", factory); err != nil {
		t.Fatal(err)
	}
	if err := o.Down(ctx, "test-project", factory); err != nil {
		t.Fatal(err)
	}
	state, err := o.store.LoadState("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if state.DiskID == "" || len(state.Snapshots) != 1 || state.Snapshots[0].Name != "custom-backup-name" {
		t.Fatalf("down lost storage or snapshot identity: %+v", state)
	}
	if containsStr(m.calls, "DeleteSnapshot") || containsStr(m.calls, "DeleteDisk") {
		t.Fatal("down must retain backups and storage")
	}
	if err := o.Destroy(ctx, "test-project", factory); err != nil {
		t.Fatal(err)
	}
	if indexOfCall(m.calls, "DeleteSnapshot") >= indexOfCall(m.calls, "DeleteDisk") || !m.deletedSnapshots["snap-789"] {
		t.Fatalf("destroy must delete backup before disk: %v", m.calls)
	}
	if _, err := os.Stat(filepath.Join(o.store.StateDir(), "test-project.json")); !os.IsNotExist(err) {
		t.Fatal("successful destroy must clear runtime state", err)
	}
	if !o.store.ProjectExists("test-project") {
		t.Fatal("destroy must keep YAML")
	}
}

func TestDestroySnapshotFailureRetainsIdentityAndDiskForRetry(t *testing.T) {
	o, m, factory := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})
	ctx := context.Background()
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.DiskID, state.DiskName = "disk-456", "serverku-test-project-data"
	if err := o.store.SaveState(state); err != nil {
		t.Fatal(err)
	}
	// These legacy snapshots were never recorded by the old backup command.
	m.listSnapshotsFunc = func(_ context.Context, q provider.SnapshotQuery) ([]provider.Snapshot, error) {
		if len(q.Disks) != 1 || q.Disks[0].ID != "disk-456" {
			t.Fatalf("wrong snapshot source: %+v", q)
		}
		var result []provider.Snapshot
		for _, snap := range []provider.Snapshot{{ID: "first", Name: "custom-one"}, {ID: "second", Name: "custom-two"}} {
			if !m.deletedSnapshots[snap.ID] {
				result = append(result, snap)
			}
		}
		return result, nil
	}
	fail := true
	m.deleteSnapshotFunc = func(_ context.Context, snap provider.Snapshot) error {
		if snap.ID == "second" && fail {
			return fmt.Errorf("permission denied")
		}
		return nil
	}
	if err := o.Destroy(ctx, "test-project", factory); err == nil {
		t.Fatal("destroy must fail when a backup cannot be removed")
	}
	state, err := o.store.LoadState("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if state.DiskID != "disk-456" || len(state.Snapshots) != 1 || state.Snapshots[0].ID != "second" {
		t.Fatalf("lost retry state: %+v", state)
	}
	if !state.CleanupPending {
		t.Fatal("failed destroy must remain explicitly pending")
	}
	if _, err := o.Up(ctx, "test-project", factory); err == nil {
		t.Fatal("up must not create resources during incomplete cleanup")
	}
	if containsStr(m.calls, "DeleteDisk") {
		t.Fatal("must not delete source disk after snapshot failure")
	}
	fail = false
	if err := o.Destroy(ctx, "test-project", factory); err != nil {
		t.Fatal(err)
	}
	if !m.deletedSnapshots["second"] || !containsStr(m.calls, "DeleteDisk") {
		t.Fatal("retry did not complete cleanup")
	}
}

func TestDestroySnapshotDiscoveryFailureLeavesDisk(t *testing.T) {
	o, m, factory := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})
	state := config.NewState("test-project", "gcp", "us-central1", "us-central1-a")
	state.DiskName = "serverku-test-project-data"
	if err := o.store.SaveState(state); err != nil {
		t.Fatal(err)
	}
	m.listSnapshotsFunc = func(context.Context, provider.SnapshotQuery) ([]provider.Snapshot, error) {
		return nil, fmt.Errorf("API unavailable")
	}
	if err := o.Destroy(context.Background(), "test-project", factory); err == nil {
		t.Fatal("discovery failure must fail destroy")
	}
	if containsStr(m.calls, "DeleteDisk") {
		t.Fatal("failed discovery must preserve source disk")
	}
}

func TestRestoreKeepsOldDiskTrackedUntilDestroy(t *testing.T) {
	o, m, factory := testSetup(t, config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"})
	ctx := context.Background()
	if _, err := o.Up(ctx, "test-project", factory); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Backup(ctx, "test-project", "before-restore", factory); err != nil {
		t.Fatal(err)
	}
	if err := o.Down(ctx, "test-project", factory); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Restore(ctx, "test-project", "before-restore", "restored", false, factory); err != nil {
		t.Fatal(err)
	}
	state, err := o.store.LoadState("test-project")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.RetainedDisks) != 1 || state.RetainedDisks[0].ID != "disk-456" || len(state.Snapshots) != 1 {
		t.Fatalf("restore lost original resources: %+v", state)
	}
	var deleted []string
	m.deleteDiskFunc = func(_ context.Context, name string) error { deleted = append(deleted, name); return nil }
	if err := o.Destroy(ctx, "test-project", factory); err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 2 || deleted[0] != "serverku-test-project-data" || deleted[1] != "restored" {
		t.Fatalf("destroy missed restored/retained disks: %v", deleted)
	}
}
