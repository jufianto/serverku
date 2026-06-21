package gcp

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"google.golang.org/api/compute/v1"
)

// SnapshotDisk creates a snapshot of the named persistent disk (in the
// provider's zone) and returns the snapshot's numeric ID.
func (g *GCPProvider) SnapshotDisk(ctx context.Context, diskName, snapshotName string) (string, error) {
	log.Printf("[gcp] creating snapshot %q of disk %q", snapshotName, diskName)

	op, err := g.service.Disks.CreateSnapshot(g.projectID, g.zone, diskName, &compute.Snapshot{Name: snapshotName}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot of disk %q: %w", diskName, err)
	}

	if err := g.waitForZoneOperation(ctx, g.projectID, g.zone, op.Name); err != nil {
		return "", fmt.Errorf("failed waiting for snapshot creation: %w", err)
	}

	created, err := g.service.Snapshots.Get(g.projectID, snapshotName).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("snapshot created but failed to retrieve details: %w", err)
	}

	log.Printf("[gcp] snapshot %q created (id: %d)", snapshotName, created.Id)
	return strconv.FormatUint(created.Id, 10), nil
}
