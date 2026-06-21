package gcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

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
