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
