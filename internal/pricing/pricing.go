package pricing

import (
	"fmt"

	"github.com/jufianto/serverku/internal/config"
)

const hoursPerMonth = 730

var vmHourlyUSD = map[string]map[string]float64{
	"digitalocean": {
		"s-1vcpu-1gb":  0.00893,
		"s-1vcpu-2gb":  0.01786,
		"s-2vcpu-2gb":  0.02679,
		"s-2vcpu-4gb":  0.03571,
		"s-4vcpu-8gb":  0.07143,
		"s-8vcpu-16gb": 0.14286,
	},
	"gcp": {
		"e2-micro":      0.00838,
		"e2-small":      0.01675,
		"e2-medium":     0.03350,
		"e2-standard-2": 0.06701,
		"e2-standard-4": 0.13401,
	},
}

var storageMonthlyPerGBUSD = map[string]float64{
	"digitalocean": 0.10,
	"gcp":          0.04,
}

// Estimate contains approximate infrastructure pricing for a project config.
type Estimate struct {
	VMHourlyUSD          float64
	DiskMonthlyUSD       float64
	MonthlyIfRunningUSD  float64
	VMPriceKnown         bool
	StoragePriceKnown    bool
	ApproximationWarning string
}

// EstimateCost returns approximate cloud costs for a project config.
func EstimateCost(cfg *config.ProjectConfig) Estimate {
	est := Estimate{StoragePriceKnown: true}

	providerVMPrices, ok := vmHourlyUSD[cfg.Provider]
	if ok {
		est.VMHourlyUSD, est.VMPriceKnown = providerVMPrices[cfg.VM.Size]
	}

	if cfg.Storage.Enabled && cfg.Storage.SizeGB > 0 {
		monthlyPerGB, ok := storageMonthlyPerGBUSD[cfg.Provider]
		est.StoragePriceKnown = ok
		if ok {
			est.DiskMonthlyUSD = float64(cfg.Storage.SizeGB) * monthlyPerGB
		}
	}

	if est.VMPriceKnown {
		est.MonthlyIfRunningUSD = est.VMHourlyUSD * hoursPerMonth
	}
	est.MonthlyIfRunningUSD += est.DiskMonthlyUSD

	if !est.VMPriceKnown || !est.StoragePriceKnown {
		est.ApproximationWarning = "price unavailable for one or more resources"
	}

	return est
}

// FormatListEstimate returns a compact estimate for tabular CLI output.
func FormatListEstimate(est Estimate, running bool) string {
	if !est.VMPriceKnown && !est.StoragePriceKnown {
		return "unknown"
	}

	if running && est.VMPriceKnown {
		return fmt.Sprintf("~$%.3f/hr + $%.2f/mo", est.VMHourlyUSD, est.DiskMonthlyUSD)
	}

	return fmt.Sprintf("~$%.2f/mo storage", est.DiskMonthlyUSD)
}

// FormatUpEstimate returns detailed estimate lines for successful up output.
func FormatUpEstimate(est Estimate) []string {
	lines := []string{}
	if est.VMPriceKnown {
		lines = append(lines, fmt.Sprintf("  Est. VM cost:      ~$%.3f/hour", est.VMHourlyUSD))
	} else {
		lines = append(lines, "  Est. VM cost:      unavailable for this size")
	}

	if est.StoragePriceKnown {
		lines = append(lines, fmt.Sprintf("  Est. storage cost: ~$%.2f/month", est.DiskMonthlyUSD))
	} else {
		lines = append(lines, "  Est. storage cost: unavailable for this provider")
	}

	if est.VMPriceKnown || est.StoragePriceKnown {
		lines = append(lines, fmt.Sprintf("  Est. running cost: ~$%.2f/month if left running", est.MonthlyIfRunningUSD))
	}

	return lines
}
