package orchestrator

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/provider"
)

// dnsMockProvider is a mockProvider that also implements provider.DNSManager.
type dnsMockProvider struct {
	*mockProvider
	ensured   []ensuredRecord
	ensureErr error
}

type ensuredRecord struct {
	fqdn string
	ip   string
	ttl  int
}

func (d *dnsMockProvider) EnsureARecord(ctx context.Context, fqdn, ip string, ttl int) error {
	d.mockProvider.calls = append(d.mockProvider.calls, "EnsureARecord")
	d.ensured = append(d.ensured, ensuredRecord{fqdn, ip, ttl})
	return d.ensureErr
}

// dnsTestSetup builds an orchestrator with a DO project that has DNS enabled and
// one router domain.
func dnsTestSetup(t *testing.T) *Orchestrator {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "serverku-dns-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	store, err := config.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.ProjectConfig{
		Name:     "dns-project",
		Provider: "digitalocean",
		Region:   "sgp1",
		VM:       config.VMConfig{Size: "s-1vcpu-1gb", Image: "ubuntu-22-04"},
		Storage:  config.StorageConfig{Enabled: false},
		Router: config.RouterConfig{
			Enabled: true,
			Domains: []config.DomainConfig{{Domain: "app.example.com", Service: "web", Upstream: "web:3000"}},
		},
		DNS: config.DNSConfig{Enabled: true, TTL: 120},
	}
	if err := store.SaveProject(cfg); err != nil {
		t.Fatal(err)
	}

	return New(store, nil, nil, nil)
}

func factoryFor(cp provider.CloudProvider) ProviderFactory {
	return func(ctx context.Context, cfg *config.ProjectConfig) (provider.CloudProvider, error) {
		return cp, nil
	}
}

func TestUp_DNSEnabled_EnsuresRecord(t *testing.T) {
	cp := &dnsMockProvider{mockProvider: &mockProvider{}}
	orch := dnsTestSetup(t)

	if _, err := orch.Up(context.Background(), "dns-project", factoryFor(cp)); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	if len(cp.ensured) != 1 {
		t.Fatalf("expected 1 ensured record, got %d: %+v", len(cp.ensured), cp.ensured)
	}
	got := cp.ensured[0]
	if got.fqdn != "app.example.com" || got.ip != "1.2.3.4" || got.ttl != 120 {
		t.Errorf("unexpected ensured record: %+v", got)
	}

	// DNS must be set after the IP is known.
	if idxOf(cp.mockProvider.calls, "EnsureARecord") < idxOf(cp.mockProvider.calls, "GetExternalIP") {
		t.Errorf("expected EnsureARecord after GetExternalIP, calls: %v", cp.mockProvider.calls)
	}
}

func TestUp_DNSEnabled_FailureTearsDownVM(t *testing.T) {
	cp := &dnsMockProvider{mockProvider: &mockProvider{}, ensureErr: errors.New("dns api down")}
	orch := dnsTestSetup(t)

	_, err := orch.Up(context.Background(), "dns-project", factoryFor(cp))
	if err == nil {
		t.Fatal("expected Up to fail when DNS update fails")
	}
	if !contains(cp.mockProvider.calls, "DestroyVM") {
		t.Errorf("expected VM to be destroyed on DNS failure, calls: %v", cp.mockProvider.calls)
	}
}

func TestUp_DNSEnabled_UnsupportedProvider(t *testing.T) {
	// Plain mockProvider does not implement provider.DNSManager.
	cp := &mockProvider{}
	orch := dnsTestSetup(t)

	_, err := orch.Up(context.Background(), "dns-project", factoryFor(cp))
	if err == nil {
		t.Fatal("expected Up to fail for a provider without DNS support")
	}
	if !contains(cp.calls, "DestroyVM") {
		t.Errorf("expected VM to be destroyed when DNS unsupported, calls: %v", cp.calls)
	}
}

func idxOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func contains(s []string, v string) bool {
	return idxOf(s, v) >= 0
}
