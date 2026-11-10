package digitalocean

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/digitalocean/godo"
)

func TestListRegions(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/regions") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"regions": []godo.Region{
				{Slug: "sgp1", Name: "Singapore 1", Available: true},
				{Slug: "nyc1", Name: "New York 1", Available: true},
				{Slug: "old1", Name: "Retired", Available: false},
			},
		})
	})

	regions, err := p.ListRegions(context.Background())
	if err != nil {
		t.Fatalf("ListRegions: %v", err)
	}
	// Unavailable regions are dropped; results are sorted by slug.
	if len(regions) != 2 {
		t.Fatalf("got %d regions, want 2: %+v", len(regions), regions)
	}
	if regions[0].Slug != "nyc1" || regions[1].Slug != "sgp1" {
		t.Errorf("regions not sorted by slug: %+v", regions)
	}
	if regions[1].Name != "Singapore 1" {
		t.Errorf("region name not mapped: %+v", regions[1])
	}
}

func TestListSizes(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sizes") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sizes": []godo.Size{
				{Slug: "s-1vcpu-2gb", Vcpus: 1, Memory: 2048, Disk: 50, PriceMonthly: 12, PriceHourly: 0.01786, Available: true, Regions: []string{"sgp1", "nyc1"}},
				{Slug: "s-1vcpu-1gb", Vcpus: 1, Memory: 1024, Disk: 25, PriceMonthly: 6, PriceHourly: 0.00893, Available: true, Regions: []string{"sgp1"}},
				{Slug: "s-nyc-only", Vcpus: 1, Memory: 1024, Disk: 25, PriceMonthly: 5, Available: true, Regions: []string{"nyc1"}},
				{Slug: "s-unavailable", Vcpus: 1, Memory: 1024, Available: false, Regions: []string{"sgp1"}},
			},
		})
	})

	sizes, err := p.ListSizes(context.Background(), "sgp1")
	if err != nil {
		t.Fatalf("ListSizes: %v", err)
	}
	// Only sizes available AND offered in sgp1: the two s-1vcpu-* ones.
	// s-nyc-only (wrong region) and s-unavailable (not available) are dropped.
	if len(sizes) != 2 {
		t.Fatalf("got %d sizes, want 2: %+v", len(sizes), sizes)
	}
	// Sorted cheapest first.
	if sizes[0].Slug != "s-1vcpu-1gb" || sizes[1].Slug != "s-1vcpu-2gb" {
		t.Errorf("sizes not sorted cheapest-first: %+v", sizes)
	}
	// Fields mapped through.
	if sizes[0].VCPUs != 1 || sizes[0].MemoryMB != 1024 || sizes[0].DiskGB != 25 || sizes[0].PriceMonthly != 6 {
		t.Errorf("size fields not mapped: %+v", sizes[0])
	}
}

func TestListSizesNoRegionFilter(t *testing.T) {
	p := pricingTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sizes": []godo.Size{
				{Slug: "a", PriceMonthly: 10, Available: true, Regions: []string{"nyc1"}},
				{Slug: "b", PriceMonthly: 4, Available: true, Regions: []string{"sgp1"}},
			},
		})
	})

	// Empty region = no filter: both returned, cheapest first.
	sizes, err := p.ListSizes(context.Background(), "")
	if err != nil {
		t.Fatalf("ListSizes: %v", err)
	}
	if len(sizes) != 2 || sizes[0].Slug != "b" {
		t.Errorf("expected both sizes, cheapest first, got %+v", sizes)
	}
}
