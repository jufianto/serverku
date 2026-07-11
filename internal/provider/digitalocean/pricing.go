package digitalocean

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/digitalocean/godo"
	"github.com/jufianto/serverku/internal/provider"
)

// Compile-time checks: the provider exposes real pricing data.
var (
	_ provider.PriceCatalog  = (*Provider)(nil)
	_ provider.UsageReporter = (*Provider)(nil)
)

// VMHourlyRateUSD implements provider.PriceCatalog using the real prices
// DigitalOcean publishes via the /v2/sizes API.
func (p *Provider) VMHourlyRateUSD(ctx context.Context, size, region string, spot bool) (float64, error) {
	if spot {
		return 0, errors.New("digitalocean has no spot pricing")
	}

	opts := &godo.ListOptions{PerPage: 200}
	for {
		sizes, resp, err := p.client.Sizes.List(ctx, opts)
		if err != nil {
			return 0, fmt.Errorf("failed to list sizes: %w", err)
		}
		for _, s := range sizes {
			if s.Slug == size {
				if s.PriceHourly <= 0 {
					return 0, fmt.Errorf("no hourly price published for size %q", size)
				}
				return s.PriceHourly, nil
			}
		}
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		page, err := resp.Links.CurrentPage()
		if err != nil {
			break
		}
		opts.Page = page + 1
	}

	return 0, fmt.Errorf("size %q not found in DigitalOcean size list", size)
}

// MonthToDateUsageUSD implements provider.UsageReporter via the
// /v2/customers/my/balance API.
func (p *Provider) MonthToDateUsageUSD(ctx context.Context) (float64, error) {
	balance, _, err := p.client.Balance.Get(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get account balance: %w", err)
	}

	usd, err := strconv.ParseFloat(balance.MonthToDateUsage, 64)
	if err != nil {
		return 0, fmt.Errorf("unexpected month_to_date_usage value %q: %w", balance.MonthToDateUsage, err)
	}
	return usd, nil
}
