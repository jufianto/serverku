package pricing

import (
	"strings"
	"testing"
	"time"

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

func TestTableRate(t *testing.T) {
	cfg := &config.ProjectConfig{Provider: "digitalocean", VM: config.VMConfig{Size: "s-1vcpu-1gb"}}
	r := TableRate(cfg)
	if !r.Known || r.Live {
		t.Errorf("expected known non-live table rate, got %+v", r)
	}
	if r.HourlyUSD != 0.00893 {
		t.Errorf("HourlyUSD = %v, want 0.00893", r.HourlyUSD)
	}

	unknown := TableRate(&config.ProjectConfig{Provider: "digitalocean", VM: config.VMConfig{Size: "nope"}})
	if unknown.Known {
		t.Errorf("expected unknown rate for unlisted size, got %+v", unknown)
	}
}

func TestAccruedUSD(t *testing.T) {
	start := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	now := start.Add(10 * time.Hour)
	if got := AccruedUSD(start, now, 0.05); got != 0.5 {
		t.Errorf("AccruedUSD = %v, want 0.5", got)
	}
	// A clock that goes backwards must not produce a negative cost.
	if got := AccruedUSD(now, start, 0.05); got != 0 {
		t.Errorf("AccruedUSD backwards = %v, want 0", got)
	}
}

func TestFormatAccruedMarksEstimates(t *testing.T) {
	start := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	now := start.Add(2 * time.Hour)

	est := FormatAccrued(Rate{HourlyUSD: 0.05, Known: true}, start, now)
	if !strings.Contains(est, "est.") || !strings.Contains(est, "~") {
		t.Errorf("table-rate output must carry est. marker: %q", est)
	}

	live := FormatAccrued(Rate{HourlyUSD: 0.05, Known: true, Live: true}, start, now)
	if strings.Contains(live, "est.") || strings.Contains(live, "~") {
		t.Errorf("live-rate output must not carry est. marker: %q", live)
	}
	if !strings.Contains(live, "$0.10") {
		t.Errorf("expected accrued $0.10 in %q", live)
	}
}
