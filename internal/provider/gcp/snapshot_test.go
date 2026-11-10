package gcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jufianto/serverku/internal/provider"
	"google.golang.org/api/compute/v1"
)

func TestSnapshotDisk_CreatesSnapshot(t *testing.T) {
	var reqName string
	g := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/operations/"):
			_ = json.NewEncoder(w).Encode(compute.Operation{Name: "op-1", Status: "DONE"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/createSnapshot"):
			b, _ := io.ReadAll(r.Body)
			var snap compute.Snapshot
			_ = json.Unmarshal(b, &snap)
			reqName = snap.Name
			_ = json.NewEncoder(w).Encode(compute.Operation{Name: "op-1", Status: "PENDING"})
		case strings.Contains(r.URL.Path, "/global/snapshots/"):
			_ = json.NewEncoder(w).Encode(compute.Snapshot{Id: 55, Name: "serverku-demo-snap"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	id, err := g.SnapshotDisk(context.Background(), "serverku-demo-data", "serverku-demo-snap")
	if err != nil {
		t.Fatalf("SnapshotDisk: %v", err)
	}
	if id != "55" {
		t.Errorf("snapshot id = %q, want 55", id)
	}
	if reqName != "serverku-demo-snap" {
		t.Errorf("snapshot request name = %q, want serverku-demo-snap", reqName)
	}
}

func TestCreateDiskFromSnapshot_SetsSourceSnapshot(t *testing.T) {
	var reqDisk compute.Disk
	g := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/operations/"):
			_ = json.NewEncoder(w).Encode(compute.Operation{Name: "op-1", Status: "DONE"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/disks"):
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &reqDisk)
			_ = json.NewEncoder(w).Encode(compute.Operation{Name: "op-1", Status: "PENDING"})
		case strings.Contains(r.URL.Path, "/disks/"):
			_ = json.NewEncoder(w).Encode(compute.Disk{Id: 77, Name: reqDisk.Name, SizeGb: 20})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	disk, err := g.CreateDiskFromSnapshot(context.Background(), provider.DiskConfig{
		Name: "serverku-demo-data-restored", Zone: "test-zone", SizeGB: 20,
	}, "serverku-demo-snap")
	if err != nil {
		t.Fatalf("CreateDiskFromSnapshot: %v", err)
	}
	if disk.ID != "77" || disk.Name != "serverku-demo-data-restored" {
		t.Errorf("unexpected disk: %+v", disk)
	}
	if !strings.HasSuffix(reqDisk.SourceSnapshot, "/global/snapshots/serverku-demo-snap") {
		t.Errorf("SourceSnapshot = %q, want .../global/snapshots/serverku-demo-snap", reqDisk.SourceSnapshot)
	}
}

func TestProjectSnapshotsMatchExactSourceAndRecordedIdentity(t *testing.T) {
	pages := 0
	g := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		if r.URL.Query().Get("pageToken") == "next" {
			_ = json.NewEncoder(w).Encode(compute.SnapshotList{Items: []*compute.Snapshot{{Id: 30, Name: "old-backup"}}})
			return
		}
		_ = json.NewEncoder(w).Encode(compute.SnapshotList{NextPageToken: "next", Items: []*compute.Snapshot{
			{Id: 10, Name: "custom", SourceDisk: "https://www.googleapis.com/compute/v1/projects/test-proj/zones/test-zone/disks/data", SourceDiskId: "disk-id"},
			{Id: 11, Name: "other-zone", SourceDisk: "https://www.googleapis.com/compute/v1/projects/test-proj/zones/other-zone/disks/data", SourceDiskId: "other"},
			{Id: 12, Name: "recreated-disk", SourceDisk: "https://www.googleapis.com/compute/v1/projects/test-proj/zones/test-zone/disks/data", SourceDiskId: "new-disk-id"},
		}})
	})
	snaps, err := g.ListProjectSnapshots(context.Background(), provider.SnapshotQuery{Disks: []provider.DiskIdentity{{ID: "disk-id", Name: "data"}}, Tracked: []provider.Snapshot{{ID: "30", Name: "old-backup"}}})
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 || len(snaps) != 2 || snaps[0].ID != "10" || snaps[1].ID != "30" {
		t.Fatalf("wrong snapshots: %+v (pages=%d)", snaps, pages)
	}
}

func TestDeleteSnapshotVerifiesIDAndWaitsForOperation(t *testing.T) {
	for _, tc := range []struct {
		name                string
		status              int
		id                  uint64
		wantErr, wantDelete bool
	}{
		{name: "delete", status: 200, id: 55, wantDelete: true},
		{name: "missing", status: 404},
		{name: "replaced identity", status: 200, id: 99, wantErr: true},
		{name: "permission error", status: 403, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleted, waited := false, false
			g := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "/operations/"):
					waited = true
					_ = json.NewEncoder(w).Encode(compute.Operation{Status: "DONE"})
				case r.Method == http.MethodDelete:
					deleted = true
					_ = json.NewEncoder(w).Encode(compute.Operation{Name: "delete-op"})
				default:
					w.WriteHeader(tc.status)
					_ = json.NewEncoder(w).Encode(compute.Snapshot{Id: tc.id, Name: "backup"})
				}
			})
			err := g.DeleteSnapshot(context.Background(), provider.Snapshot{ID: "55", Name: "backup"})
			if (err != nil) != tc.wantErr || deleted != tc.wantDelete || waited != tc.wantDelete {
				t.Fatalf("err=%v deleted=%v waited=%v", err, deleted, waited)
			}
		})
	}
}
