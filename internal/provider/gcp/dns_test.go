package gcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dns "google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

func TestCanonicalName(t *testing.T) {
	cases := map[string]string{
		"app.example.com":    "app.example.com.",
		"app.example.com.":   "app.example.com.",
		"  APP.Example.COM ": "app.example.com.",
	}
	for in, want := range cases {
		if got := canonicalName(in); got != want {
			t.Errorf("canonicalName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchManagedZone(t *testing.T) {
	managed := map[string]string{
		"z-root":    "example.com.",
		"z-staging": "staging.example.com.",
		"z-other":   "other.org.",
	}
	cases := []struct {
		name     string
		wantZone string
		wantErr  bool
	}{
		{"app.example.com.", "z-root", false},
		{"example.com.", "z-root", false},
		{"api.staging.example.com.", "z-staging", false}, // longest-suffix wins
		{"notexample.com.", "", true},                    // label-boundary: must not match example.com.
		{"app.unmanaged.net.", "", true},
	}
	for _, tc := range cases {
		got, err := matchManagedZone(tc.name, managed)
		if tc.wantErr {
			if err == nil {
				t.Errorf("matchManagedZone(%q): expected error, got %q", tc.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("matchManagedZone(%q): unexpected error: %v", tc.name, err)
			continue
		}
		if got != tc.wantZone {
			t.Errorf("matchManagedZone(%q) = %q, want %q", tc.name, got, tc.wantZone)
		}
	}
}

// dnsFake simulates the Cloud DNS API. existing is returned for rrset lists;
// change captures the applied Changes.Create body.
type dnsFake struct {
	zones    []*dns.ManagedZone
	existing []*dns.ResourceRecordSet
	change   *dns.Change
}

func newDNSProvider(t *testing.T, f *dnsFake) *GCPProvider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/managedZones"):
			_ = json.NewEncoder(w).Encode(dns.ManagedZonesListResponse{ManagedZones: f.zones})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rrsets"):
			_ = json.NewEncoder(w).Encode(dns.ResourceRecordSetsListResponse{Rrsets: f.existing})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/changes"):
			var c dns.Change
			_ = json.NewDecoder(r.Body).Decode(&c)
			f.change = &c
			c.Status = "done"
			_ = json.NewEncoder(w).Encode(c)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	svc, err := dns.NewService(context.Background(),
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("dns.NewService: %v", err)
	}
	return NewWithServices(nil, svc, "test-proj", "test-zone")
}

func TestEnsureARecord_CreatesWhenMissing(t *testing.T) {
	f := &dnsFake{zones: []*dns.ManagedZone{{Name: "z-root", DnsName: "example.com."}}}
	g := newDNSProvider(t, f)

	if err := g.EnsureARecord(context.Background(), "app.example.com", "203.0.113.5", 300); err != nil {
		t.Fatalf("EnsureARecord: %v", err)
	}
	if f.change == nil || len(f.change.Additions) != 1 {
		t.Fatalf("expected one addition, got change: %+v", f.change)
	}
	add := f.change.Additions[0]
	if add.Name != "app.example.com." || add.Type != "A" || add.Ttl != 300 || add.Rrdatas[0] != "203.0.113.5" {
		t.Errorf("unexpected addition: %+v", add)
	}
	if len(f.change.Deletions) != 0 {
		t.Errorf("did not expect deletions for a new record, got %+v", f.change.Deletions)
	}
}

func TestEnsureARecord_UpdatesWhenChanged(t *testing.T) {
	f := &dnsFake{
		zones:    []*dns.ManagedZone{{Name: "z-root", DnsName: "example.com."}},
		existing: []*dns.ResourceRecordSet{{Name: "app.example.com.", Type: "A", Ttl: 300, Rrdatas: []string{"198.51.100.1"}}},
	}
	g := newDNSProvider(t, f)

	if err := g.EnsureARecord(context.Background(), "app.example.com", "203.0.113.5", 300); err != nil {
		t.Fatalf("EnsureARecord: %v", err)
	}
	if f.change == nil || len(f.change.Deletions) != 1 || len(f.change.Additions) != 1 {
		t.Fatalf("expected an update (1 deletion + 1 addition), got: %+v", f.change)
	}
	if f.change.Additions[0].Rrdatas[0] != "203.0.113.5" {
		t.Errorf("expected new IP in addition, got %+v", f.change.Additions[0])
	}
}

func TestEnsureARecord_NoopWhenUnchanged(t *testing.T) {
	f := &dnsFake{
		zones:    []*dns.ManagedZone{{Name: "z-root", DnsName: "example.com."}},
		existing: []*dns.ResourceRecordSet{{Name: "app.example.com.", Type: "A", Ttl: 300, Rrdatas: []string{"203.0.113.5"}}},
	}
	g := newDNSProvider(t, f)

	if err := g.EnsureARecord(context.Background(), "app.example.com", "203.0.113.5", 300); err != nil {
		t.Fatalf("EnsureARecord: %v", err)
	}
	if f.change != nil {
		t.Errorf("expected no change for a matching record, got %+v", f.change)
	}
}

func TestEnsureARecord_NoManagedZone(t *testing.T) {
	f := &dnsFake{zones: []*dns.ManagedZone{{Name: "z-other", DnsName: "other.org."}}}
	g := newDNSProvider(t, f)

	err := g.EnsureARecord(context.Background(), "app.example.com", "203.0.113.5", 300)
	if err == nil {
		t.Fatal("expected error when no managed zone matches")
	}
	if !strings.Contains(err.Error(), "no Cloud DNS managed zone") {
		t.Errorf("unexpected error: %v", err)
	}
}
