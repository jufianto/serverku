package gcp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jufianto/serverku/internal/provider"
	"google.golang.org/api/cloudbilling/v1"
)

// Compile-time check: the provider exposes real pricing data.
var _ provider.PriceCatalog = (*GCPProvider)(nil)

// computeServiceName is the stable Cloud Billing Catalog identifier for the
// Compute Engine service.
const computeServiceName = "services/6F81-5844-456A"

// machineShape is the billable decomposition of a machine type: GCP prices
// predefined instances as core-hours plus RAM GiB-hours.
type machineShape struct {
	descKeyword string  // SKU description keyword for the family, e.g. "E2 Instance"
	cores       float64 // billed core-equivalents (shared-core types bill fractions)
	ramGB       float64
}

// decomposeMachineType maps a machine type name to its billable shape.
// Families not listed here return an error, which callers treat as "fall back
// to the offline table estimate".
func decomposeMachineType(size string) (machineShape, error) {
	// Shared-core e2 types bill fractional core-equivalents.
	switch size {
	case "e2-micro":
		return machineShape{"E2 Instance", 0.25, 1}, nil
	case "e2-small":
		return machineShape{"E2 Instance", 0.5, 2}, nil
	case "e2-medium":
		return machineShape{"E2 Instance", 1, 4}, nil
	}

	parts := strings.Split(size, "-")
	if len(parts) != 3 {
		return machineShape{}, fmt.Errorf("unsupported machine type %q", size)
	}

	family, class := parts[0], parts[1]
	n, err := strconv.Atoi(parts[2])
	if err != nil || n <= 0 {
		return machineShape{}, fmt.Errorf("unsupported machine type %q", size)
	}
	cores := float64(n)

	keywords := map[string]string{
		"e2":  "E2 Instance",
		"n1":  "N1 Predefined Instance",
		"n2":  "N2 Instance",
		"n2d": "N2D AMD Instance",
	}
	keyword, ok := keywords[family]
	if !ok {
		return machineShape{}, fmt.Errorf("unsupported machine family %q", family)
	}

	// RAM per core by class. N1 uses 3.75 GB/core for standard.
	var ramPerCore float64
	switch class {
	case "standard":
		ramPerCore = 4
		if family == "n1" {
			ramPerCore = 3.75
		}
	case "highmem":
		ramPerCore = 8
		if family == "n1" {
			ramPerCore = 6.5
		}
	case "highcpu":
		ramPerCore = 1
		if family == "n1" {
			ramPerCore = 0.9
		}
	default:
		return machineShape{}, fmt.Errorf("unsupported machine class %q", class)
	}

	return machineShape{keyword, cores, cores * ramPerCore}, nil
}

// VMHourlyRateUSD implements provider.PriceCatalog using the Cloud Billing
// Catalog API: the machine type is decomposed into core-hours and RAM
// GiB-hours and priced with the region's published SKUs (spot SKUs when spot
// is true). Requires the Cloud Billing API to be enabled; callers fall back
// to labeled offline estimates when it is not.
func (g *GCPProvider) VMHourlyRateUSD(ctx context.Context, size, region string, spot bool) (float64, error) {
	if g.billingService == nil {
		return 0, errors.New("cloud billing service unavailable")
	}

	shape, err := decomposeMachineType(size)
	if err != nil {
		return 0, err
	}

	corePerHour, ramPerGBHour, err := g.findComputeRates(ctx, shape.descKeyword, region, spot)
	if err != nil {
		return 0, err
	}

	return shape.cores*corePerHour + shape.ramGB*ramPerGBHour, nil
}

// findComputeRates scans the Compute Engine SKU catalog for the family's Core
// and Ram SKUs in the given region, honoring spot (Preemptible) usage type.
func (g *GCPProvider) findComputeRates(ctx context.Context, keyword, region string, spot bool) (corePerHour, ramPerGBHour float64, err error) {
	usageType := "OnDemand"
	if spot {
		usageType = "Preemptible"
	}

	var coreFound, ramFound bool
	call := g.billingService.Services.Skus.List(computeServiceName).Context(ctx).PageSize(5000)
	err = call.Pages(ctx, func(resp *cloudbilling.ListSkusResponse) error {
		for _, sku := range resp.Skus {
			if sku.Category == nil ||
				sku.Category.ResourceFamily != "Compute" ||
				sku.Category.UsageType != usageType {
				continue
			}
			if !slices.Contains(sku.ServiceRegions, region) {
				continue
			}
			desc := sku.Description
			if !strings.Contains(desc, keyword) || strings.Contains(desc, "Custom") ||
				strings.Contains(desc, "Sole Tenancy") {
				continue
			}

			price, ok := skuHourlyUSD(sku)
			if !ok {
				continue
			}
			switch {
			case strings.Contains(desc, keyword+" Core"):
				corePerHour, coreFound = price, true
			case strings.Contains(desc, keyword+" Ram"):
				ramPerGBHour, ramFound = price, true
			}
			if coreFound && ramFound {
				return errStopPaging
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopPaging) {
		return 0, 0, fmt.Errorf("failed to list compute SKUs: %w", err)
	}

	if !coreFound || !ramFound {
		return 0, 0, fmt.Errorf("no %s Core/Ram SKUs found for region %s (usage type %s)", keyword, region, usageType)
	}
	return corePerHour, ramPerGBHour, nil
}

// errStopPaging aborts SKU pagination early once both rates are found.
var errStopPaging = errors.New("stop paging")

// skuHourlyUSD extracts the effective per-unit hourly USD price from a SKU.
func skuHourlyUSD(sku *cloudbilling.Sku) (float64, bool) {
	if len(sku.PricingInfo) == 0 {
		return 0, false
	}
	pe := sku.PricingInfo[0].PricingExpression
	if pe == nil || len(pe.TieredRates) == 0 {
		return 0, false
	}
	rate := pe.TieredRates[len(pe.TieredRates)-1]
	if rate.UnitPrice == nil {
		return 0, false
	}
	return float64(rate.UnitPrice.Units) + float64(rate.UnitPrice.Nanos)/1e9, true
}
