//go:build floci

package gcp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jufianto/serverku/internal/provider"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// This opt-in test uses only the local Floci Compute endpoint and synthetic resources.
func TestFlociDiskAndFirewallLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	svc, err := compute.NewService(ctx, option.WithEndpoint("http://127.0.0.1:4588/compute/v1/"), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	project := fmt.Sprintf("serverku-probe-%d", time.Now().UnixNano())
	g := NewWithService(svc, project, "us-central1-a")
	if err := g.ValidateCredentials(ctx); err != nil {
		t.Fatal(err)
	}
	t.Log("Compute regions: PASS")
	disk, err := g.CreateDisk(ctx, provider.DiskConfig{Name: "data", SizeGB: 10})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := g.DeleteDisk(context.Background(), "data"); err != nil {
			t.Error(err)
		}
	})
	if disk.Name != "data" || disk.ID == "" || disk.SizeGB != 10 {
		t.Fatalf("unexpected disk: %+v", disk)
	}
	t.Log("Create/read disk: PASS")
	op, err := svc.Networks.Insert(project, &compute.Network{Name: "default", AutoCreateSubnetworks: false, ForceSendFields: []string{"AutoCreateSubnetworks"}}).Context(ctx).Do()
	if err != nil {
		t.Fatal(err)
	}
	if err := g.waitForGlobalOperation(ctx, project, op.Name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := svc.Networks.Delete(project, "default").Do(); err != nil {
			t.Error(err)
		}
	})
	if err := g.EnsureFirewall(ctx, "probe"); err != nil {
		t.Error(err)
	} else {
		t.Log("Create firewall: PASS")
		if err := g.EnsureFirewall(ctx, "probe"); err != nil {
			t.Error(err)
		}
		if err := g.DeleteFirewall(ctx, "probe"); err != nil {
			t.Error(err)
		}
	}
}
