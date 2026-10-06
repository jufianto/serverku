package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/provider"
)

type inventoryProvider struct {
	provider.CloudProvider
	query      provider.ComponentQuery
	components []provider.Component
}

func (p *inventoryProvider) ListComponents(_ context.Context, q provider.ComponentQuery) ([]provider.Component, error) {
	p.query = q
	return p.components, nil
}

func TestComponentsShowsSharedKeyWithoutOrphanWarning(t *testing.T) {
	s := withStore(t)
	key, err := s.ResolveProjectSSHKey(&config.ProjectConfig{Name: "kuma"}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := key.PublicKey
	p := &inventoryProvider{components: []provider.Component{
		{Kind: "SSH key", Name: "serverku-serverku-wpblog", Present: true, Shared: true},
		{Kind: "Snapshot", Detail: "1 found", Present: true},
	}}
	out, err := os.CreateTemp(t.TempDir(), "status-output")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = out.Close() })
	previous := os.Stdout
	os.Stdout = out
	t.Cleanup(func() { os.Stdout = previous })
	printComponents(context.Background(), &config.ProjectConfig{Name: "kuma"}, &config.ProjectState{}, func(context.Context, *config.ProjectConfig) (provider.CloudProvider, error) { return p, nil })
	os.Stdout = previous
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	if p.query.SSHPubKey != publicKey || p.query.VMName != "serverku-kuma" {
		t.Fatal("inventory did not receive the local public key and VM name")
	}
	if !strings.Contains(output, "serverku-serverku-wpblog") || !strings.Contains(output, "shared (retained by destroy)") || !strings.Contains(output, "1 component(s)") || strings.Contains(output, "2 component(s)") {
		t.Fatalf("shared key must not count as an orphan: %s", output)
	}
}
