package gcp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/jufianto/serverku/internal/provider"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
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

// CreateDiskFromSnapshot creates a new persistent disk restored from a
// snapshot, referenced by its name.
func (g *GCPProvider) CreateDiskFromSnapshot(ctx context.Context, cfg provider.DiskConfig, snapshot string) (*provider.Disk, error) {
	zone := g.resolveZone(cfg.Zone)
	projectID := g.resolveProjectID(cfg.ProjectID)

	diskType := cfg.DiskType
	if diskType == "" {
		diskType = defaultDataDiskType
	}

	disk := &compute.Disk{
		Name:           cfg.Name,
		SizeGb:         cfg.SizeGB,
		Type:           fmt.Sprintf("zones/%s/diskTypes/%s", zone, diskType),
		SourceSnapshot: fmt.Sprintf("projects/%s/global/snapshots/%s", projectID, snapshot),
	}

	log.Printf("[gcp] creating disk %q from snapshot %q in zone %s", cfg.Name, snapshot, zone)

	op, err := g.service.Disks.Insert(projectID, zone, disk).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create disk from snapshot %q: %w", snapshot, err)
	}

	if err := g.waitForZoneOperation(ctx, projectID, zone, op.Name); err != nil {
		return nil, fmt.Errorf("failed waiting for disk restore: %w", err)
	}

	created, err := g.service.Disks.Get(projectID, zone, cfg.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("disk restored but failed to retrieve details: %w", err)
	}

	return &provider.Disk{
		ID:       strconv.FormatUint(created.Id, 10),
		Name:     created.Name,
		Zone:     zone,
		SizeGB:   created.SizeGb,
		Provider: "gcp",
	}, nil
}

func (g *GCPProvider) ListProjectSnapshots(ctx context.Context, q provider.SnapshotQuery) ([]provider.Snapshot, error) {
	tracked := map[string]bool{}
	for _, snap := range q.Tracked {
		if snap.ID != "" {
			tracked[snap.ID] = true
		}
	}
	var result []provider.Snapshot
	err := g.service.Snapshots.List(g.projectID).Context(ctx).Pages(ctx, func(list *compute.SnapshotList) error {
		for _, s := range list.Items {
			id := strconv.FormatUint(s.Id, 10)
			matched := tracked[id]
			for _, disk := range q.Disks {
				source := fmt.Sprintf("projects/%s/zones/%s/disks/%s", g.projectID, g.zone, disk.Name)
				// SourceDiskId prevents matching a different disk recreated under the same name.
				if disk.Name != "" && (s.SourceDisk == source || strings.HasSuffix(s.SourceDisk, "/"+source)) && (disk.ID == "" || s.SourceDiskId == disk.ID) {
					matched = true
				}
			}
			if matched {
				result = append(result, provider.Snapshot{ID: id, Name: s.Name})
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list project snapshots: %w", err)
	}
	return result, nil
}

func (g *GCPProvider) DeleteSnapshot(ctx context.Context, snapshot provider.Snapshot) error {
	if snapshot.Name == "" || snapshot.ID == "" {
		return fmt.Errorf("snapshot name and ID are required")
	}
	existing, err := g.service.Snapshots.Get(g.projectID, snapshot.Name).Context(ctx).Do()
	if snapshotNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect snapshot: %w", err)
	}
	if strconv.FormatUint(existing.Id, 10) != snapshot.ID {
		return fmt.Errorf("snapshot %q identity changed; refusing deletion", snapshot.Name)
	}
	log.Printf("[gcp] deleting snapshot %q (id: %s)", snapshot.Name, snapshot.ID)
	op, err := g.service.Snapshots.Delete(g.projectID, snapshot.Name).Context(ctx).Do()
	if snapshotNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete snapshot: %w", err)
	}
	return g.waitForGlobalOperation(ctx, g.projectID, op.Name)
}

func snapshotNotFound(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == 404
}
