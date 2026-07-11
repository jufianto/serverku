package gcp

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/api/cloudbilling/v1"
	"google.golang.org/api/option"
)

func fakeBillingProvider(t *testing.T, handler http.HandlerFunc) *GCPProvider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	svc, err := cloudbilling.NewService(context.Background(),
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("cloudbilling.NewService: %v", err)
	}
	return NewWithBillingService(svc, "test-proj", "test-zone")
}

func billingSku(desc, usageType, region string, units int64, nanos int64) *cloudbilling.Sku {
	return &cloudbilling.Sku{
		Description: desc,
		Category: &cloudbilling.Category{
			ResourceFamily: "Compute",
			UsageType:      usageType,
		},
		ServiceRegions: []string{region},
		PricingInfo: []*cloudbilling.PricingInfo{{
			PricingExpression: &cloudbilling.PricingExpression{
				TieredRates: []*cloudbilling.TierRate{{
					UnitPrice: &cloudbilling.Money{CurrencyCode: "USD", Units: units, Nanos: nanos},
				}},
			},
		}},
	}
}

func skusHandler(skus []*cloudbilling.Sku) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(cloudbilling.ListSkusResponse{Skus: skus})
	}
}

func TestVMHourlyRateUSD_E2Medium(t *testing.T) {
	p := fakeBillingProvider(t, skusHandler([]*cloudbilling.Sku{
		// Distractors that must be skipped: wrong region, custom, spot.
		billingSku("E2 Instance Core running in Jakarta", "OnDemand", "asia-southeast2", 0, 30000000),
		billingSku("E2 Custom Instance Core running in Singapore", "OnDemand", "asia-southeast1", 0, 99000000),
		billingSku("Spot Preemptible E2 Instance Core running in Singapore", "Preemptible", "asia-southeast1", 0, 6000000),
		// The two SKUs that should be used.
		billingSku("E2 Instance Core running in Singapore", "OnDemand", "asia-southeast1", 0, 24000000), // $0.024/core-hr
		billingSku("E2 Instance Ram running in Singapore", "OnDemand", "asia-southeast1", 0, 3200000),   // $0.0032/GiB-hr
	}))

	// e2-medium bills as 1 core-equivalent + 4 GB.
	rate, err := p.VMHourlyRateUSD(context.Background(), "e2-medium", "asia-southeast1", false)
	if err != nil {
		t.Fatalf("VMHourlyRateUSD: %v", err)
	}
	want := 1*0.024 + 4*0.0032
	if math.Abs(rate-want) > 1e-9 {
		t.Errorf("rate = %v, want %v", rate, want)
	}
}

func TestVMHourlyRateUSD_SpotUsesPreemptibleSKUs(t *testing.T) {
	p := fakeBillingProvider(t, skusHandler([]*cloudbilling.Sku{
		billingSku("E2 Instance Core running in Singapore", "OnDemand", "asia-southeast1", 0, 24000000),
		billingSku("E2 Instance Ram running in Singapore", "OnDemand", "asia-southeast1", 0, 3200000),
		billingSku("Spot Preemptible E2 Instance Core running in Singapore", "Preemptible", "asia-southeast1", 0, 6000000),
		billingSku("Spot Preemptible E2 Instance Ram running in Singapore", "Preemptible", "asia-southeast1", 0, 800000),
	}))

	rate, err := p.VMHourlyRateUSD(context.Background(), "e2-small", "asia-southeast1", true)
	if err != nil {
		t.Fatalf("VMHourlyRateUSD: %v", err)
	}
	// e2-small bills as 0.5 core-equivalents + 2 GB at spot rates.
	want := 0.5*0.006 + 2*0.0008
	if math.Abs(rate-want) > 1e-9 {
		t.Errorf("rate = %v, want %v", rate, want)
	}
}

func TestVMHourlyRateUSD_MissingSKUsErrors(t *testing.T) {
	p := fakeBillingProvider(t, skusHandler([]*cloudbilling.Sku{
		billingSku("E2 Instance Core running in Singapore", "OnDemand", "asia-southeast1", 0, 24000000),
		// No Ram SKU for the region.
	}))

	if _, err := p.VMHourlyRateUSD(context.Background(), "e2-medium", "asia-southeast1", false); err == nil {
		t.Fatal("expected error when Ram SKU is missing")
	}
}

func TestVMHourlyRateUSD_UnsupportedFamilyErrors(t *testing.T) {
	p := fakeBillingProvider(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no API call expected for unsupported machine type")
	})

	if _, err := p.VMHourlyRateUSD(context.Background(), "m3-ultramem-32", "asia-southeast1", false); err == nil {
		t.Fatal("expected error for unsupported machine family")
	}
}

func TestDecomposeMachineType(t *testing.T) {
	cases := []struct {
		size  string
		cores float64
		ramGB float64
	}{
		{"e2-micro", 0.25, 1},
		{"e2-small", 0.5, 2},
		{"e2-medium", 1, 4},
		{"e2-standard-4", 4, 16},
		{"e2-highmem-2", 2, 16},
		{"e2-highcpu-8", 8, 8},
		{"n1-standard-1", 1, 3.75},
		{"n2-standard-2", 2, 8},
	}
	for _, c := range cases {
		shape, err := decomposeMachineType(c.size)
		if err != nil {
			t.Errorf("%s: %v", c.size, err)
			continue
		}
		if shape.cores != c.cores || shape.ramGB != c.ramGB {
			t.Errorf("%s: got %v cores / %v GB, want %v / %v", c.size, shape.cores, shape.ramGB, c.cores, c.ramGB)
		}
	}

	if _, err := decomposeMachineType("t2d-standard-1"); err == nil {
		t.Error("expected error for unsupported family")
	}
}
