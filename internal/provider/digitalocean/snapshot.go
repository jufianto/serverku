package digitalocean

import (
	"context"
	"fmt"
	"log"

	"github.com/digitalocean/godo"
	"github.com/jufianto/serverku/internal/provider"
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

// CreateDiskFromSnapshot creates a new Block Storage volume restored from a
// volume snapshot, referenced by name or ID.
func (p *Provider) CreateDiskFromSnapshot(ctx context.Context, cfg provider.DiskConfig, snapshot string) (*provider.Disk, error) {
	snapshotID, err := p.resolveVolumeSnapshotID(ctx, snapshot)
	if err != nil {
		return nil, err
	}

	log.Printf("[do] creating volume %q from snapshot %q", cfg.Name, snapshot)

	// Region is inherited from the snapshot when SnapshotID is set.
	vol, _, err := p.client.Storage.CreateVolume(ctx, &godo.VolumeCreateRequest{
		Name:          cfg.Name,
		SizeGigaBytes: cfg.SizeGB,
		SnapshotID:    snapshotID,
		Description:   "serverku persistent storage (restored)",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create volume from snapshot %q: %w", snapshot, err)
	}

	return &provider.Disk{
		ID:       vol.ID,
		Name:     vol.Name,
		Zone:     vol.Region.Slug,
		SizeGB:   vol.SizeGigaBytes,
		Provider: "digitalocean",
	}, nil
}

// resolveVolumeSnapshotID maps a snapshot name (or ID) to the snapshot ID the
// volumes API requires.
func (p *Provider) resolveVolumeSnapshotID(ctx context.Context, snapshot string) (string, error) {
	opts := &godo.ListOptions{PerPage: 200}
	for {
		snapshots, resp, err := p.client.Snapshots.ListVolume(ctx, opts)
		if err != nil {
			return "", fmt.Errorf("failed to list volume snapshots: %w", err)
		}
		for _, s := range snapshots {
			if s.ID == snapshot || s.Name == snapshot {
				return s.ID, nil
			}
		}
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		page, err := resp.Links.CurrentPage()
		if err != nil {
			break
		}
		opts.Page = page + 1
	}
	return "", fmt.Errorf("volume snapshot %q not found", snapshot)
}
