package digitalocean

import (
	"context"
	"fmt"
	"sort"

	"github.com/digitalocean/godo"
	"github.com/jufianto/serverku/internal/provider"
)

// Compile-time check: the provider can enumerate regions and sizes for `init`.
var _ provider.CatalogLister = (*Provider)(nil)

// ListRegions implements provider.CatalogLister, returning DigitalOcean's
// available regions (slug + human name).
func (p *Provider) ListRegions(ctx context.Context) ([]provider.CatalogRegion, error) {
	regions, _, err := p.client.Regions.List(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return nil, fmt.Errorf("failed to list regions: %w", err)
	}
	out := make([]provider.CatalogRegion, 0, len(regions))
	for _, r := range regions {
		if !r.Available {
			continue
		}
		out = append(out, provider.CatalogRegion{Slug: r.Slug, Name: r.Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// ListSizes implements provider.CatalogLister, returning DigitalOcean's
// available sizes offered in the given region (empty region = no filter),
// cheapest first.
func (p *Provider) ListSizes(ctx context.Context, region string) ([]provider.CatalogSize, error) {
	var out []provider.CatalogSize
	opts := &godo.ListOptions{PerPage: 200}
	for {
		sizes, resp, err := p.client.Sizes.List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list sizes: %w", err)
		}
		for _, s := range sizes {
			if !s.Available {
				continue
			}
			if region != "" && !regionOffersSize(s, region) {
				continue
			}
			out = append(out, provider.CatalogSize{
				Slug:         s.Slug,
				VCPUs:        s.Vcpus,
				MemoryMB:     s.Memory,
				DiskGB:       s.Disk,
				PriceMonthly: s.PriceMonthly,
				PriceHourly:  s.PriceHourly,
			})
		}
		if resp == nil || resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		page, err := resp.Links.CurrentPage()
		if err != nil {
			break
		}
		opts.Page = page + 1
	}

	sort.Slice(out, func(i, j int) bool { return out[i].PriceMonthly < out[j].PriceMonthly })
	return out, nil
}

// regionOffersSize reports whether a size is available in the given region.
func regionOffersSize(s godo.Size, region string) bool {
	for _, r := range s.Regions {
		if r == region {
			return true
		}
	}
	return false
}
