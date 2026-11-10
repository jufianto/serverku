package digitalocean

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAttachDiskByIDSkipsNameLookup(t *testing.T) {
	var attached bool
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/droplets":
			_, _ = w.Write([]byte(`{"droplets":[{"id":42,"name":"serverku-kuma","region":{"slug":"sgp1"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v2/volumes":
			t.Error("attachment must not depend on the volume listing")
			_, _ = w.Write([]byte(`{"volumes":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v2/volumes/saved-volume-id/actions":
			var body struct {
				Type      string `json:"type"`
				DropletID int    `json:"droplet_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Type != "attach" || body.DropletID != 42 {
				t.Errorf("unexpected action: %+v", body)
			}
			attached = true
			_, _ = w.Write([]byte(`{"action":{"id":7,"status":"in-progress"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v2/volumes/saved-volume-id/actions/7":
			_, _ = w.Write([]byte(`{"action":{"id":7,"status":"completed"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	})
	if err := p.AttachDiskByID(context.Background(), "serverku-kuma", "saved-volume-id"); err != nil {
		t.Fatal(err)
	}
	if !attached {
		t.Fatal("saved volume ID was not attached")
	}
}

func TestAttachDiskByIDReportsMissingVolume(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/droplets" {
			_, _ = w.Write([]byte(`{"droplets":[{"id":42,"name":"serverku-kuma","region":{"slug":"sgp1"}}]}`))
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v2/volumes/missing-id/actions" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"id":"not_found","message":"volume not found"}`))
	})
	err := p.AttachDiskByID(context.Background(), "serverku-kuma", "missing-id")
	if err == nil || !strings.Contains(err.Error(), "missing-id") || !strings.Contains(err.Error(), "volume not found") {
		t.Fatalf("missing volume error: %v", err)
	}
}
