package digitalocean

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/digitalocean/godo"
	"github.com/jufianto/serverku/internal/provider"
)

func TestSnapshotDisk_ResolvesVolumeAndSnapshots(t *testing.T) {
	var createBody godo.SnapshotCreateRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/volumes"):
			// getVolumeIDByName lists volumes filtered by name.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"volumes": []godo.Volume{{ID: "vol-1", Name: "serverku-demo-data"}},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/snapshots"):
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &createBody)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"snapshot": godo.Snapshot{ID: "snap-1", Name: createBody.Name},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := godo.New(srv.Client(), godo.SetBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatalf("godo.New: %v", err)
	}
	p := &Provider{client: client}

	id, err := p.SnapshotDisk(context.Background(), "serverku-demo-data", "serverku-demo-snap")
	if err != nil {
		t.Fatalf("SnapshotDisk: %v", err)
	}
	if id != "snap-1" {
		t.Errorf("snapshot id = %q, want snap-1", id)
	}
	if createBody.VolumeID != "vol-1" || createBody.Name != "serverku-demo-snap" {
		t.Errorf("unexpected snapshot request: %+v", createBody)
	}
}

func TestCreateDiskFromSnapshot_ResolvesNameAndCreates(t *testing.T) {
	var createReq godo.VolumeCreateRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/snapshots"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"snapshots": []godo.Snapshot{{ID: "snap-1", Name: "serverku-demo-backup"}},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/volumes"):
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &createReq)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"volume": godo.Volume{ID: "vol-new", Name: createReq.Name, SizeGigaBytes: createReq.SizeGigaBytes, Region: &godo.Region{Slug: "sgp1"}},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := godo.New(srv.Client(), godo.SetBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatalf("godo.New: %v", err)
	}
	p := &Provider{client: client}

	// Reference the snapshot by name; the provider must resolve it to the ID.
	disk, err := p.CreateDiskFromSnapshot(context.Background(), provider.DiskConfig{
		Name: "serverku-demo-data-restored", Zone: "sgp1", SizeGB: 20,
	}, "serverku-demo-backup")
	if err != nil {
		t.Fatalf("CreateDiskFromSnapshot: %v", err)
	}
	if disk.ID != "vol-new" || disk.Name != "serverku-demo-data-restored" {
		t.Errorf("unexpected disk: %+v", disk)
	}
	if createReq.SnapshotID != "snap-1" {
		t.Errorf("create request SnapshotID = %q, want snap-1 (resolved from name)", createReq.SnapshotID)
	}
}

func TestCreateDiskFromSnapshot_UnknownSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"snapshots": []godo.Snapshot{}})
	}))
	t.Cleanup(srv.Close)

	client, _ := godo.New(srv.Client(), godo.SetBaseURL(srv.URL+"/"))
	p := &Provider{client: client}

	if _, err := p.CreateDiskFromSnapshot(context.Background(), provider.DiskConfig{Name: "d", Zone: "sgp1", SizeGB: 20}, "nope"); err == nil {
		t.Fatal("expected error for unknown snapshot")
	}
}
