package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/jufianto/serverku/internal/provider"
	"github.com/jufianto/serverku/internal/provider/digitalocean"
)

// initCatalog holds the region/size choices offered during interactive init,
// sourced live from the provider when possible and from a built-in curated list
// otherwise.
type initCatalog struct {
	provider string
	regions  []provider.CatalogRegion
	sizesFor func(region string) []provider.CatalogSize
	live     bool
}

// loadCatalog builds the region/size catalog for a provider. It queries the
// live provider API when credentials allow (currently DigitalOcean, which needs
// only a token), and falls back to a curated static list otherwise -- so init
// works before `serverku setup` and offline.
func loadCatalog(providerName, projectID string) *initCatalog {
	ctx := context.Background()

	if providerName == "digitalocean" {
		if token, err := resolveDOToken(); err == nil && token != "" {
			if p, err := digitalocean.NewWithToken(token); err == nil {
				if regions, err := p.ListRegions(ctx); err == nil && len(regions) > 0 {
					cache := map[string][]provider.CatalogSize{}
					sizesFor := func(region string) []provider.CatalogSize {
						if v, ok := cache[region]; ok {
							return v
						}
						sizes, err := p.ListSizes(ctx, region)
						if err != nil || len(sizes) == 0 {
							sizes = staticSizes[providerName]
						}
						sizes = curateSizes(providerName, sizes)
						cache[region] = sizes
						return sizes
					}
					return &initCatalog{provider: providerName, regions: regions, sizesFor: sizesFor, live: true}
				}
			}
		}
	}

	// Static fallback (also the only path for GCP in this version).
	return &initCatalog{
		provider: providerName,
		regions:  staticRegions[providerName],
		sizesFor: func(string) []provider.CatalogSize { return staticSizes[providerName] },
		live:     false,
	}
}

// regionOptions renders the catalog regions as huh select options.
func (c *initCatalog) regionOptions() []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(c.regions))
	for _, r := range c.regions {
		label := r.Slug
		if r.Name != "" {
			label = fmt.Sprintf("%s — %s", r.Slug, r.Name)
		}
		opts = append(opts, huh.NewOption(label, r.Slug))
	}
	return opts
}

// sizeOptions renders the sizes available in a region as huh select options.
func (c *initCatalog) sizeOptions(region string) []huh.Option[string] {
	sizes := c.sizesFor(region)
	opts := make([]huh.Option[string], 0, len(sizes))
	for _, s := range sizes {
		opts = append(opts, huh.NewOption(sizeLabel(s), s.Slug))
	}
	return opts
}

// sizeLabel renders a human-readable, priced label for a size option.
func sizeLabel(s provider.CatalogSize) string {
	label := s.Slug + "  —  " + fmt.Sprintf("%d vCPU · %s", s.VCPUs, formatMB(s.MemoryMB))
	if s.DiskGB > 0 {
		label += fmt.Sprintf(" · %d GB disk", s.DiskGB)
	}
	if s.PriceMonthly > 0 {
		label += fmt.Sprintf(" · $%g/mo", s.PriceMonthly)
	}
	return label
}

// formatMB renders memory in MB as a friendly GB/MB string.
func formatMB(mb int) string {
	switch {
	case mb >= 1024 && mb%1024 == 0:
		return fmt.Sprintf("%d GB", mb/1024)
	case mb >= 1024:
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	default:
		return fmt.Sprintf("%d MB", mb)
	}
}

// curateSizes trims a provider's full size list to a sensible shortlist for the
// picker. For DigitalOcean that's the basic shared-CPU droplets up to 8 GB (the
// premium -intel/-amd variants and large sizes are hidden to keep the list
// approachable; edit the YAML for anything else).
func curateSizes(providerName string, sizes []provider.CatalogSize) []provider.CatalogSize {
	if providerName != "digitalocean" {
		return sizes
	}
	var out []provider.CatalogSize
	for _, s := range sizes {
		if !strings.HasPrefix(s.Slug, "s-") {
			continue
		}
		if strings.Contains(s.Slug, "-intel") || strings.Contains(s.Slug, "-amd") {
			continue
		}
		if s.MemoryMB > 8192 {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return sizes // never over-filter to an empty list
	}
	return out
}

// staticRegions / staticSizes are the built-in fallback catalog used when the
// live provider API is unavailable. DigitalOcean prices are the real published
// rates; GCP prices vary by region and are left unset (0 = not shown).
var staticRegions = map[string][]provider.CatalogRegion{
	"digitalocean": {
		{Slug: "sgp1", Name: "Singapore 1"},
		{Slug: "nyc1", Name: "New York 1"},
		{Slug: "nyc3", Name: "New York 3"},
		{Slug: "ams3", Name: "Amsterdam 3"},
		{Slug: "fra1", Name: "Frankfurt 1"},
		{Slug: "lon1", Name: "London 1"},
		{Slug: "sfo3", Name: "San Francisco 3"},
		{Slug: "blr1", Name: "Bangalore 1"},
		{Slug: "syd1", Name: "Sydney 1"},
		{Slug: "tor1", Name: "Toronto 1"},
	},
	"gcp": {
		{Slug: "asia-southeast1", Name: "Singapore"},
		{Slug: "us-central1", Name: "Iowa"},
		{Slug: "us-east1", Name: "South Carolina"},
		{Slug: "europe-west1", Name: "Belgium"},
		{Slug: "europe-west2", Name: "London"},
		{Slug: "asia-south1", Name: "Mumbai"},
		{Slug: "australia-southeast1", Name: "Sydney"},
	},
}

var staticSizes = map[string][]provider.CatalogSize{
	"digitalocean": {
		{Slug: "s-1vcpu-512mb-10gb", VCPUs: 1, MemoryMB: 512, DiskGB: 10, PriceMonthly: 4},
		{Slug: "s-1vcpu-1gb", VCPUs: 1, MemoryMB: 1024, DiskGB: 25, PriceMonthly: 6},
		{Slug: "s-1vcpu-2gb", VCPUs: 1, MemoryMB: 2048, DiskGB: 50, PriceMonthly: 12},
		{Slug: "s-2vcpu-2gb", VCPUs: 2, MemoryMB: 2048, DiskGB: 60, PriceMonthly: 18},
		{Slug: "s-2vcpu-4gb", VCPUs: 2, MemoryMB: 4096, DiskGB: 80, PriceMonthly: 24},
		{Slug: "s-4vcpu-8gb", VCPUs: 4, MemoryMB: 8192, DiskGB: 160, PriceMonthly: 48},
	},
	"gcp": {
		{Slug: "e2-micro", VCPUs: 2, MemoryMB: 1024},
		{Slug: "e2-small", VCPUs: 2, MemoryMB: 2048},
		{Slug: "e2-medium", VCPUs: 2, MemoryMB: 4096},
		{Slug: "e2-standard-2", VCPUs: 2, MemoryMB: 8192},
		{Slug: "n2-standard-2", VCPUs: 2, MemoryMB: 8192},
	},
}
