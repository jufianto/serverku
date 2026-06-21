package digitalocean

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/digitalocean/godo"
)

// EnsureARecord creates or updates an A record for fqdn pointing at ip.
//
// The fqdn must fall under a domain already managed in the DigitalOcean account
// (its nameservers delegated to DigitalOcean). The record is idempotent: if a
// matching A record already points at ip, it is left unchanged.
func (p *Provider) EnsureARecord(ctx context.Context, fqdn, ip string, ttl int) error {
	domains, _, err := p.client.Domains.List(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return fmt.Errorf("failed to list DigitalOcean domains: %w", err)
	}

	managed := make([]string, len(domains))
	for i, d := range domains {
		managed[i] = d.Name
	}

	zone, recordName, err := splitDomain(fqdn, managed)
	if err != nil {
		return err
	}

	records, _, err := p.client.Domains.RecordsByType(ctx, zone, "A", &godo.ListOptions{PerPage: 200})
	if err != nil {
		return fmt.Errorf("failed to list A records for %q: %w", zone, err)
	}

	req := &godo.DomainRecordEditRequest{
		Type: "A",
		Name: recordName,
		Data: ip,
		TTL:  ttl,
	}

	for _, r := range records {
		if r.Name != recordName {
			continue
		}
		if r.Data == ip {
			log.Printf("[do] DNS A record %s already points at %s", fqdn, ip)
			return nil
		}
		log.Printf("[do] updating DNS A record %s -> %s (was %s)", fqdn, ip, r.Data)
		if _, _, err := p.client.Domains.EditRecord(ctx, zone, r.ID, req); err != nil {
			return fmt.Errorf("failed to update A record for %q: %w", fqdn, err)
		}
		return nil
	}

	log.Printf("[do] creating DNS A record %s -> %s", fqdn, ip)
	if _, _, err := p.client.Domains.CreateRecord(ctx, zone, req); err != nil {
		return fmt.Errorf("failed to create A record for %q: %w", fqdn, err)
	}
	return nil
}

// splitDomain splits a fully-qualified name into the managed zone it belongs to
// and the relative record name within that zone. It picks the longest managed
// domain that is a suffix of fqdn (matching on label boundaries), so that
// "api.staging.example.com" prefers a managed "staging.example.com" zone over
// "example.com". The record name for an apex match is "@".
func splitDomain(fqdn string, managed []string) (zone, recordName string, err error) {
	fqdn = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(fqdn), "."))

	best := ""
	for _, m := range managed {
		m = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(m), "."))
		if fqdn == m || strings.HasSuffix(fqdn, "."+m) {
			if len(m) > len(best) {
				best = m
			}
		}
	}

	if best == "" {
		return "", "", fmt.Errorf("no DigitalOcean-managed domain found for %q (the domain's nameservers must be delegated to DigitalOcean)", fqdn)
	}

	if fqdn == best {
		return best, "@", nil
	}
	return best, strings.TrimSuffix(fqdn, "."+best), nil
}
