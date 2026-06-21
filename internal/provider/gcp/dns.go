package gcp

import (
	"context"
	"fmt"
	"log"
	"strings"

	dns "google.golang.org/api/dns/v1"
)

// EnsureARecord creates or updates an A record for fqdn pointing at ip, using
// Cloud DNS. The fqdn must fall under a managed zone in the project. The change
// is idempotent: a record already pointing at ip with the same TTL is left
// unchanged.
func (g *GCPProvider) EnsureARecord(ctx context.Context, fqdn, ip string, ttl int) error {
	name := canonicalName(fqdn) // Cloud DNS uses fully-qualified names with a trailing dot.

	zones, err := g.dnsService.ManagedZones.List(g.projectID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to list Cloud DNS managed zones: %w", err)
	}

	managed := make(map[string]string, len(zones.ManagedZones)) // zone name -> dnsName
	for _, z := range zones.ManagedZones {
		managed[z.Name] = z.DnsName
	}

	zoneName, err := matchManagedZone(name, managed)
	if err != nil {
		return err
	}

	desired := &dns.ResourceRecordSet{
		Name:    name,
		Type:    "A",
		Ttl:     int64(ttl),
		Rrdatas: []string{ip},
	}

	existing, err := g.dnsService.ResourceRecordSets.List(g.projectID, zoneName).
		Name(name).Type("A").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to list A records in zone %q: %w", zoneName, err)
	}

	change := &dns.Change{Additions: []*dns.ResourceRecordSet{desired}}
	if len(existing.Rrsets) > 0 {
		current := existing.Rrsets[0]
		if recordMatches(current, ip, int64(ttl)) {
			log.Printf("[gcp] DNS A record %s already points at %s", fqdn, ip)
			return nil
		}
		log.Printf("[gcp] updating DNS A record %s -> %s (was %v)", fqdn, ip, current.Rrdatas)
		change.Deletions = []*dns.ResourceRecordSet{current}
	} else {
		log.Printf("[gcp] creating DNS A record %s -> %s", fqdn, ip)
	}

	if _, err := g.dnsService.Changes.Create(g.projectID, zoneName, change).Context(ctx).Do(); err != nil {
		return fmt.Errorf("failed to apply DNS change for %q: %w", fqdn, err)
	}
	return nil
}

// canonicalName lowercases fqdn and ensures a single trailing dot, the form
// Cloud DNS uses for record and zone names.
func canonicalName(fqdn string) string {
	f := strings.ToLower(strings.TrimSpace(fqdn))
	if !strings.HasSuffix(f, ".") {
		f += "."
	}
	return f
}

// matchManagedZone returns the zone name whose dnsName is the longest suffix of
// the canonical (trailing-dot) record name. managed maps zone name -> dnsName.
func matchManagedZone(name string, managed map[string]string) (string, error) {
	bestZone, bestLen := "", -1
	for zoneName, dnsName := range managed {
		dnsName = strings.ToLower(dnsName)
		// dnsName carries a trailing dot (e.g. "example.com."). Match on label
		// boundaries: the apex itself, or any subdomain of it.
		if name == dnsName || strings.HasSuffix(name, "."+dnsName) {
			if len(dnsName) > bestLen {
				bestZone, bestLen = zoneName, len(dnsName)
			}
		}
	}
	if bestZone == "" {
		return "", fmt.Errorf("no Cloud DNS managed zone found for %q in this project", strings.TrimSuffix(name, "."))
	}
	return bestZone, nil
}

// recordMatches reports whether rrset already encodes exactly the given ip and ttl.
func recordMatches(rrset *dns.ResourceRecordSet, ip string, ttl int64) bool {
	return rrset.Ttl == ttl && len(rrset.Rrdatas) == 1 && rrset.Rrdatas[0] == ip
}
