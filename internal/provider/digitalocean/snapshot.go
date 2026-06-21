package digitalocean

import (
	"context"
	"fmt"
	"log"

	"github.com/digitalocean/godo"
)

// SnapshotDisk creates a snapshot of the named Block Storage volume and returns
// the snapshot's ID.
func (p *Provider) SnapshotDisk(ctx context.Context, diskName, snapshotName string) (string, error) {
	volID, err := p.getVolumeIDByName(ctx, diskName)
	if err != nil {
		return "", err
	}

	log.Printf("[do] creating snapshot %q of volume %q", snapshotName, diskName)

	snapshot, _, err := p.client.Storage.CreateSnapshot(ctx, &godo.SnapshotCreateRequest{
		VolumeID: volID,
		Name:     snapshotName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot of volume %q: %w", diskName, err)
	}

	log.Printf("[do] snapshot %q created (id: %s)", snapshotName, snapshot.ID)
	return snapshot.ID, nil
}
