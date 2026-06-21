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

func TestSplitDomain(t *testing.T) {
	managed := []string{"example.com", "staging.example.com", "other.org"}

	cases := []struct {
		fqdn       string
		wantZone   string
		wantRecord string
		wantErr    bool
	}{
		{"app.example.com", "example.com", "app", false},
		{"example.com", "example.com", "@", false},
		{"a.b.example.com", "example.com", "a.b", false},
		// longest-suffix preference: should pick staging.example.com over example.com
		{"api.staging.example.com", "staging.example.com", "api", false},
		{"APP.Example.com.", "example.com", "app", false}, // case + trailing dot
		{"app.notmanaged.net", "", "", true},
	}

	for _, tc := range cases {
		zone, rec, err := splitDomain(tc.fqdn, managed)
		if tc.wantErr {
			if err == nil {
				t.Errorf("splitDomain(%q): expected error, got zone=%q rec=%q", tc.fqdn, zone, rec)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitDomain(%q): unexpected error: %v", tc.fqdn, err)
			continue
		}
		if zone != tc.wantZone || rec != tc.wantRecord {
			t.Errorf("splitDomain(%q) = (%q, %q), want (%q, %q)", tc.fqdn, zone, rec, tc.wantZone, tc.wantRecord)
		}
	}
}

// dnsServer simulates the DigitalOcean Domains API. existing is the set of A
// records returned for the zone; created/edited capture mutations.
type dnsServer struct {
	domains  []string
	existing []godo.DomainRecord
	created  *godo.DomainRecordEditRequest
	edited   *godo.DomainRecordEditRequest
}

func newDNSProvider(t *testing.T, s *dnsServer) *Provider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/domains":
			ds := make([]godo.Domain, len(s.domains))
			for i, d := range s.domains {
				ds[i] = godo.Domain{Name: d}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"domains": ds})

		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/records"):
			_ = json.NewEncoder(w).Encode(map[string]any{"domain_records": s.existing})

		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/records"):
			body, _ := io.ReadAll(r.Body)
			var req godo.DomainRecordEditRequest
			_ = json.Unmarshal(body, &req)
			s.created = &req
			_ = json.NewEncoder(w).Encode(map[string]any{"domain_record": godo.DomainRecord{ID: 99, Type: req.Type, Name: req.Name, Data: req.Data}})

		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/records/"):
			body, _ := io.ReadAll(r.Body)
			var req godo.DomainRecordEditRequest
			_ = json.Unmarshal(body, &req)
			s.edited = &req
			_ = json.NewEncoder(w).Encode(map[string]any{"domain_record": godo.DomainRecord{ID: 1, Type: req.Type, Name: req.Name, Data: req.Data}})

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := godo.New(srv.Client(), godo.SetBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatalf("godo.New: %v", err)
	}
	return &Provider{client: client}
}

func TestEnsureARecord_CreatesWhenMissing(t *testing.T) {
	s := &dnsServer{domains: []string{"example.com"}}
	p := newDNSProvider(t, s)

	if err := p.EnsureARecord(context.Background(), "app.example.com", "203.0.113.5", 3600); err != nil {
		t.Fatalf("EnsureARecord: %v", err)
	}

	if s.created == nil {
		t.Fatal("expected a record to be created")
	}
	if s.created.Name != "app" || s.created.Data != "203.0.113.5" || s.created.Type != "A" || s.created.TTL != 3600 {
		t.Errorf("unexpected create request: %+v", s.created)
	}
	if s.edited != nil {
		t.Errorf("did not expect an edit, got %+v", s.edited)
	}
}

func TestEnsureARecord_UpdatesWhenChanged(t *testing.T) {
	s := &dnsServer{
		domains:  []string{"example.com"},
		existing: []godo.DomainRecord{{ID: 1, Type: "A", Name: "app", Data: "198.51.100.1"}},
	}
	p := newDNSProvider(t, s)

	if err := p.EnsureARecord(context.Background(), "app.example.com", "203.0.113.5", 60); err != nil {
		t.Fatalf("EnsureARecord: %v", err)
	}

	if s.edited == nil {
		t.Fatal("expected the existing record to be updated")
	}
	if s.edited.Data != "203.0.113.5" {
		t.Errorf("expected updated data 203.0.113.5, got %q", s.edited.Data)
	}
	if s.created != nil {
		t.Errorf("did not expect a create, got %+v", s.created)
	}
}

func TestEnsureARecord_NoopWhenUnchanged(t *testing.T) {
	s := &dnsServer{
		domains:  []string{"example.com"},
		existing: []godo.DomainRecord{{ID: 1, Type: "A", Name: "app", Data: "203.0.113.5"}},
	}
	p := newDNSProvider(t, s)

	if err := p.EnsureARecord(context.Background(), "app.example.com", "203.0.113.5", 3600); err != nil {
		t.Fatalf("EnsureARecord: %v", err)
	}

	if s.created != nil || s.edited != nil {
		t.Errorf("expected no mutation for matching record, got created=%+v edited=%+v", s.created, s.edited)
	}
}

func TestEnsureARecord_NoManagedDomain(t *testing.T) {
	s := &dnsServer{domains: []string{"other.org"}}
	p := newDNSProvider(t, s)

	err := p.EnsureARecord(context.Background(), "app.example.com", "203.0.113.5", 3600)
	if err == nil {
		t.Fatal("expected error when no managed domain matches, got nil")
	}
	if !strings.Contains(err.Error(), "no DigitalOcean-managed domain") {
		t.Errorf("unexpected error: %v", err)
	}
}
