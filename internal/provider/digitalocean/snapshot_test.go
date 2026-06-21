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
