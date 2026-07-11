package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/orchestrator"
	"github.com/jufianto/serverku/internal/pricing"
	"github.com/jufianto/serverku/internal/provider"
)

// rateLookupTimeout bounds a single live pricing API call so commands like
// `list` never hang on a slow provider endpoint.
const rateLookupTimeout = 5 * time.Second

// rateCache resolves VM hourly rates, preferring the provider's live pricing
// API (PriceCatalog capability) and falling back to the offline tables. Both
// rates and provider clients are memoized for the lifetime of one CLI
// invocation so `list` does at most one API round-trip per provider.
type rateCache struct {
	factory   orchestrator.ProviderFactory
	rates     map[string]pricing.Rate
	providers map[string]provider.CloudProvider
}

func newRateCache(factory orchestrator.ProviderFactory) *rateCache {
	return &rateCache{
		factory:   factory,
		rates:     map[string]pricing.Rate{},
		providers: map[string]provider.CloudProvider{},
	}
}

// providerFor returns a memoized provider client for the project's cloud.
func (c *rateCache) providerFor(ctx context.Context, cfg *config.ProjectConfig) (provider.CloudProvider, error) {
	key := cfg.Provider + "|" + cfg.ProjectID
	if cp, ok := c.providers[key]; ok {
		if cp == nil {
			return nil, fmt.Errorf("provider %q previously failed to construct", cfg.Provider)
		}
		return cp, nil
	}
	cp, err := c.factory(ctx, cfg)
	if err != nil {
		c.providers[key] = nil
		return nil, err
	}
	c.providers[key] = cp
	return cp, nil
}

// resolveRate returns the best available hourly rate for a project's VM.
// Live API prices are preferred; the offline table is the fallback and is
// labeled as an estimate everywhere it is displayed.
func (c *rateCache) resolveRate(ctx context.Context, cfg *config.ProjectConfig) pricing.Rate {
	key := fmt.Sprintf("%s|%s|%s|%v", cfg.Provider, cfg.Region, cfg.VM.Size, cfg.VM.Spot)
	if r, ok := c.rates[key]; ok {
		return r
	}

	rate := pricing.TableRate(cfg)
	if cp, err := c.providerFor(ctx, cfg); err == nil {
		if pc, ok := cp.(provider.PriceCatalog); ok {
			lookupCtx, cancel := context.WithTimeout(ctx, rateLookupTimeout)
			hourly, err := pc.VMHourlyRateUSD(lookupCtx, cfg.VM.Size, cfg.Region, cfg.VM.Spot)
			cancel()
			if err == nil {
				rate = pricing.Rate{HourlyUSD: hourly, Live: true, Known: true}
			} else if verbose {
				log.Printf("[cost] live rate lookup failed for %s/%s, using offline estimate: %v", cfg.Provider, cfg.VM.Size, err)
			}
		}
	}

	c.rates[key] = rate
	return rate
}

// monthToDateUsage returns the provider's real account-level month-to-date
// usage when the provider supports it (UsageReporter capability).
func (c *rateCache) monthToDateUsage(ctx context.Context, cfg *config.ProjectConfig) (float64, bool) {
	cp, err := c.providerFor(ctx, cfg)
	if err != nil {
		return 0, false
	}
	ur, ok := cp.(provider.UsageReporter)
	if !ok {
		return 0, false
	}
	lookupCtx, cancel := context.WithTimeout(ctx, rateLookupTimeout)
	defer cancel()
	usd, err := ur.MonthToDateUsageUSD(lookupCtx)
	if err != nil {
		if verbose {
			log.Printf("[cost] month-to-date usage lookup failed for %s: %v", cfg.Provider, err)
		}
		return 0, false
	}
	return usd, true
}
