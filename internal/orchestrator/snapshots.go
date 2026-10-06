package orchestrator

import (
	"context"
	"fmt"
	"log"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/provider"
)

func projectSnapshotQuery(state *config.ProjectState) provider.SnapshotQuery {
	q := provider.SnapshotQuery{}
	if state.DiskName != "" || state.DiskID != "" {
		q.Disks = append(q.Disks, provider.DiskIdentity{ID: state.DiskID, Name: state.DiskName})
	}
	for _, disk := range state.RetainedDisks {
		q.Disks = append(q.Disks, provider.DiskIdentity{ID: disk.ID, Name: disk.Name})
	}
	for _, snap := range state.Snapshots {
		q.Tracked = append(q.Tracked, provider.Snapshot{ID: snap.ID, Name: snap.Name})
	}
	return q
}

func (o *Orchestrator) destroySnapshots(ctx context.Context, cp provider.CloudProvider, state *config.ProjectState) error {
	q := projectSnapshotQuery(state)
	if len(q.Disks) == 0 && len(q.Tracked) == 0 {
		return nil
	}
	manager, ok := cp.(provider.SnapshotManager)
	if !ok {
		return fmt.Errorf("provider cannot clean up project snapshots; runtime state retained")
	}
	snapshots, err := manager.ListProjectSnapshots(ctx, q)
	if err != nil {
		return fmt.Errorf("failed to discover project snapshots: %w", err)
	}
	// Persist legacy discoveries before deleting their source disks. A partial
	// cleanup must remain retryable, even if snapshots use custom names.
	state.Snapshots = nil
	for _, snap := range snapshots {
		state.Snapshots = append(state.Snapshots, config.ResourceIdentity{ID: snap.ID, Name: snap.Name})
	}
	if err := o.store.SaveState(state); err != nil {
		return fmt.Errorf("failed to record snapshots for cleanup: %w", err)
	}
	for len(state.Snapshots) > 0 {
		snap := state.Snapshots[0]
		log.Printf("[orchestrator] deleting snapshot %q (id: %s)", snap.Name, snap.ID)
		if err := manager.DeleteSnapshot(ctx, provider.Snapshot{ID: snap.ID, Name: snap.Name}); err != nil {
			return fmt.Errorf("failed to delete snapshot %q (id: %s): %w", snap.Name, snap.ID, err)
		}
		state.Snapshots = state.Snapshots[1:]
		if err := o.store.SaveState(state); err != nil {
			return fmt.Errorf("failed to save snapshot cleanup state: %w", err)
		}
	}
	return nil
}

func deleteTrackedDisk(ctx context.Context, cp provider.CloudProvider, id, name string) error {
	if manager, ok := cp.(provider.DiskDeleterByID); ok && id != "" {
		return manager.DeleteDiskByID(ctx, id)
	}
	if name == "" {
		return fmt.Errorf("disk name is required for this provider")
	}
	return cp.DeleteDisk(ctx, name)
}
