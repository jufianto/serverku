package pricing

import (
	"testing"

	"github.com/jufianto/serverku/internal/config"
)

func TestEstimateCostDigitalOcean(t *testing.T) {
	cfg := &config.ProjectConfig{
		Provider: "digitalocean",
		VM:       config.VMConfig{Size: "s-1vcpu-1gb"},
		Storage:  config.StorageConfig{Enabled: true, SizeGB: 10},
	}

	est := EstimateCost(cfg)
	if !est.VMPriceKnown {
		t.Fatal("expected DigitalOcean VM price to be known")
	}
	if !est.StoragePriceKnown {
		t.Fatal("expected DigitalOcean storage price to be known")
	}
	if est.VMHourlyUSD != 0.00893 {
		t.Fatalf("expected VM hourly 0.00893, got %f", est.VMHourlyUSD)
	}
	if est.DiskMonthlyUSD != 1.0 {
		t.Fatalf("expected disk monthly 1.0, got %f", est.DiskMonthlyUSD)
	}
}

func TestEstimateCostGCP(t *testing.T) {
	cfg := &config.ProjectConfig{
		Provider: "gcp",
		VM:       config.VMConfig{Size: "e2-medium"},
		Storage:  config.StorageConfig{Enabled: true, SizeGB: 20},
	}

	est := EstimateCost(cfg)
	if !est.VMPriceKnown {
		t.Fatal("expected GCP VM price to be known")
	}
	if !est.StoragePriceKnown {
		t.Fatal("expected GCP storage price to be known")
	}
	if est.VMHourlyUSD != 0.03350 {
		t.Fatalf("expected VM hourly 0.03350, got %f", est.VMHourlyUSD)
	}
	if est.DiskMonthlyUSD != 0.8 {
		t.Fatalf("expected disk monthly 0.8, got %f", est.DiskMonthlyUSD)
	}
}

func TestEstimateCostUnknownSizeAndDisabledStorage(t *testing.T) {
	cfg := &config.ProjectConfig{
		Provider: "digitalocean",
		VM:       config.VMConfig{Size: "unknown-size"},
		Storage:  config.StorageConfig{Enabled: false},
	}

	est := EstimateCost(cfg)
	if est.VMPriceKnown {
		t.Fatal("expected unknown VM price")
	}
	if !est.StoragePriceKnown {
		t.Fatal("expected disabled storage to be treated as known zero cost")
	}
	if est.DiskMonthlyUSD != 0 {
		t.Fatalf("expected disabled storage to cost 0, got %f", est.DiskMonthlyUSD)
	}
	if est.ApproximationWarning == "" {
		t.Fatal("expected warning for unknown VM price")
	}
}
