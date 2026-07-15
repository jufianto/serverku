package digitalocean

import (
	"context"
	"os"
	"testing"

	"github.com/digitalocean/godo"
	"github.com/jufianto/serverku/internal/provider"
)

func TestNew_MissingToken(t *testing.T) {
	// Temporarily unset token
	oldToken := os.Getenv("DIGITALOCEAN_TOKEN")
	os.Unsetenv("DIGITALOCEAN_TOKEN")
	defer os.Setenv("DIGITALOCEAN_TOKEN", oldToken)

	_, err := New(context.Background())
	if err == nil {
		t.Error("expected error when DIGITALOCEAN_TOKEN is not set, got nil")
	}
}

func TestCreateVM_SpotNotSupported(t *testing.T) {
	// Since we mock New without a token, we just create an empty provider
	p := &Provider{}

	_, err := p.CreateVM(context.Background(), provider.VMConfig{
		Spot: true,
	})
	if err == nil {
		t.Error("expected error for spot instance request, got nil")
	}
	if err.Error() != "DigitalOcean provider does not support spot instances" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetDropletPublicIPv4(t *testing.T) {
	droplet := &godo.Droplet{
		Networks: &godo.Networks{
			V4: []godo.NetworkV4{
				{
					Type:      "private",
					IPAddress: "10.0.0.1",
				},
				{
					Type:      "public",
					IPAddress: "192.168.1.1",
				},
			},
		},
	}

	ip, err := getDropletPublicIPv4(droplet)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ip != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %s", ip)
	}

	dropletNoPublic := &godo.Droplet{
		Networks: &godo.Networks{
			V4: []godo.NetworkV4{
				{
					Type:      "private",
					IPAddress: "10.0.0.1",
				},
			},
		},
	}
	_, err = getDropletPublicIPv4(dropletNoPublic)
	if err == nil {
		t.Error("expected error when no public IP is available")
	}
}

func TestNewWithToken(t *testing.T) {
	if _, err := NewWithToken(""); err == nil {
		t.Error("expected error for empty token")
	}
	p, err := NewWithToken("dop_v1_token")
	if err != nil {
		t.Fatalf("NewWithToken: %v", err)
	}
	if p == nil || p.client == nil {
		t.Error("expected a provider with an initialized client")
	}
}
