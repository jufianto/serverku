package digitalocean

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/jufianto/serverku/internal/provider"
)

// Provider implements the CloudProvider interface for DigitalOcean.
type Provider struct {
	client *godo.Client
}

// New creates a new DigitalOcean provider instance.
// It requires the DIGITALOCEAN_TOKEN environment variable to be set.
func New(ctx context.Context) (*Provider, error) {
	token := os.Getenv("DIGITALOCEAN_TOKEN")
	if token == "" {
		return nil, errors.New("DIGITALOCEAN_TOKEN environment variable is not set")
	}

	client := godo.NewFromToken(token)

	return &Provider{
		client: client,
	}, nil
}

// CreateVM creates a new Droplet.
func (p *Provider) CreateVM(ctx context.Context, cfg provider.VMConfig) (*provider.VM, error) {
	if cfg.Spot {
		return nil, errors.New("DigitalOcean provider does not support spot instances")
	}

	// For DigitalOcean, we need to create an SSH key first or find an existing one by name/fingerprint.
	// Since we are given the public key string directly in VMConfig, the easiest approach
	// is to create a new SSH key in DO, use it for the Droplet, and then we might leave it
	// or clean it up. A better approach for serverku is to ensure a key named "serverku-projectname" exists.
	keyName := fmt.Sprintf("serverku-%s", cfg.Name)

	// Try to find if the key already exists
	keys, _, err := p.client.Keys.List(ctx, &godo.ListOptions{PerPage: 100})
	if err != nil {
		return nil, fmt.Errorf("failed to list SSH keys: %w", err)
	}

	var sshKeyID int
	var sshKeyFingerprint string
	for _, k := range keys {
		if k.Name == keyName {
			sshKeyID = k.ID
			sshKeyFingerprint = k.Fingerprint
			break
		}
	}

	if sshKeyID == 0 {
		// Create the key
		req := &godo.KeyCreateRequest{
			Name:      keyName,
			PublicKey: cfg.SSHPubKey,
		}
		k, _, err := p.client.Keys.Create(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH key in DigitalOcean: %w", err)
		}
		sshKeyID = k.ID
		sshKeyFingerprint = k.Fingerprint
	}

	createRequest := &godo.DropletCreateRequest{
		Name:   cfg.Name,
		Region: cfg.Region,
		Size:   cfg.MachineType,
		Image: godo.DropletCreateImage{
			Slug: cfg.Image,
		},
		SSHKeys: []godo.DropletCreateSSHKey{
			{ID: sshKeyID, Fingerprint: sshKeyFingerprint},
		},
		Tags: cfg.Tags,
	}

	droplet, _, err := p.client.Droplets.Create(ctx, createRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create droplet: %w", err)
	}

	return &provider.VM{
		ID:       fmt.Sprintf("%d", droplet.ID),
		Name:     droplet.Name,
		Zone:     cfg.Region,
		Provider: "digitalocean",
	}, nil
}

// GetVM retrieves a Droplet by name.
func (p *Provider) GetVM(ctx context.Context, name string) (*provider.VM, error) {
	droplets, _, err := p.client.Droplets.ListByTag(ctx, fmt.Sprintf("serverku-%s", strings.TrimPrefix(name, "serverku-")), &godo.ListOptions{PerPage: 10})
	if err != nil {
		return nil, fmt.Errorf("failed to get droplets by tag: %w", err)
	}

	for _, d := range droplets {
		if d.Name == name {
			return &provider.VM{
				ID:       fmt.Sprintf("%d", d.ID),
				Name:     d.Name,
				Zone:     d.Region.Slug,
				Provider: "digitalocean",
			}, nil
		}
	}

	return nil, fmt.Errorf("droplet %q not found", name)
}

// GetVMStatus retrieves the status of a Droplet.
func (p *Provider) GetVMStatus(ctx context.Context, name string) (*provider.VMStatus, error) {
	droplets, _, err := p.client.Droplets.ListByTag(ctx, fmt.Sprintf("serverku-%s", strings.TrimPrefix(name, "serverku-")), &godo.ListOptions{PerPage: 10})
	if err != nil {
		return nil, fmt.Errorf("failed to get droplets by tag: %w", err)
	}

	for _, d := range droplets {
		if d.Name == name {
			state := provider.VMStateUnknown
			switch d.Status {
			case "new":
				state = provider.VMStateStarting
			case "active":
				state = provider.VMStateRunning
			case "off":
				state = provider.VMStateStopped
			}

			ip, _ := getDropletPublicIPv4(&d)

			return &provider.VMStatus{
				State:      state,
				ExternalIP: ip,
			}, nil
		}
	}

	return &provider.VMStatus{State: provider.VMStateTerminated}, nil
}

// getDropletPublicIPv4 extracts the public IPv4 address from a Droplet.
func getDropletPublicIPv4(d *godo.Droplet) (string, error) {
	for _, v4 := range d.Networks.V4 {
		if v4.Type == "public" {
			return v4.IPAddress, nil
		}
	}
	return "", errors.New("no public IPv4 address found")
}

// GetExternalIP retrieves the public IPv4 address of a Droplet.
func (p *Provider) GetExternalIP(ctx context.Context, name string) (string, error) {
	droplets, _, err := p.client.Droplets.ListByTag(ctx, fmt.Sprintf("serverku-%s", strings.TrimPrefix(name, "serverku-")), &godo.ListOptions{PerPage: 10})
	if err != nil {
		return "", fmt.Errorf("failed to get droplets by tag: %w", err)
	}

	for _, d := range droplets {
		if d.Name == name {
			return getDropletPublicIPv4(&d)
		}
	}
	return "", fmt.Errorf("droplet %q not found", name)
}

// StopVM shuts down a Droplet.
func (p *Provider) StopVM(ctx context.Context, name string) error {
	vm, err := p.GetVM(ctx, name)
	if err != nil {
		return err
	}

	// We need to parse the ID back to int
	var id int
	_, _ = fmt.Sscanf(vm.ID, "%d", &id)

	action, _, err := p.client.DropletActions.PowerOff(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to power off droplet: %w", err)
	}

	// Wait for action to complete? For Stop we might just return
	_ = action
	return nil
}

// StartVM starts a previously stopped Droplet.
func (p *Provider) StartVM(ctx context.Context, name string) error {
	vm, err := p.GetVM(ctx, name)
	if err != nil {
		return err
	}

	var id int
	_, _ = fmt.Sscanf(vm.ID, "%d", &id)

	action, _, err := p.client.DropletActions.PowerOn(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to power on droplet: %w", err)
	}

	_ = action
	return nil
}

// DestroyVM permanently deletes a Droplet.
func (p *Provider) DestroyVM(ctx context.Context, name string) error {
	vm, err := p.GetVM(ctx, name)
	if err != nil {
		// If it's already gone, consider it a success
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}

	var id int
	_, _ = fmt.Sscanf(vm.ID, "%d", &id)

	_, err = p.client.Droplets.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete droplet: %w", err)
	}
	return nil
}

// WaitForReady blocks until the Droplet is in an active state.
func (p *Provider) WaitForReady(ctx context.Context, name string) error {
	maxRetries := 60 // 60 * 5s = 5 minutes
	for i := 0; i < maxRetries; i++ {
		status, err := p.GetVMStatus(ctx, name)
		if err != nil {
			log.Printf("[digitalocean] error checking droplet status: %v", err)
		} else if status.State == provider.VMStateRunning {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("timeout waiting for droplet %q to be ready", name)
}

// CreateDisk creates a new Block Storage volume.
func (p *Provider) CreateDisk(ctx context.Context, cfg provider.DiskConfig) (*provider.Disk, error) {
	req := &godo.VolumeCreateRequest{
		Name:          cfg.Name,
		Region:        cfg.Zone, // Use zone as region for DO
		SizeGigaBytes: cfg.SizeGB,
		Description:   "serverku persistent storage",
	}

	vol, _, err := p.client.Storage.CreateVolume(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	return &provider.Disk{
		ID:       vol.ID,
		Name:     vol.Name,
		Zone:     cfg.Zone,
		SizeGB:   cfg.SizeGB,
		Provider: "digitalocean",
	}, nil
}

// getDropletIDByName is a helper to find Droplet ID by name
func (p *Provider) getDropletIDByName(ctx context.Context, name string) (int, error) {
	vm, err := p.GetVM(ctx, name)
	if err != nil {
		return 0, err
	}
	var id int
	_, _ = fmt.Sscanf(vm.ID, "%d", &id)
	return id, nil
}

// getVolumeIDByName is a helper to find Volume ID by name
func (p *Provider) getVolumeIDByName(ctx context.Context, name string) (string, error) {
	vols, _, err := p.client.Storage.ListVolumes(ctx, &godo.ListVolumeParams{
		Name: name,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list volumes: %w", err)
	}
	if len(vols) == 0 {
		return "", fmt.Errorf("volume %q not found", name)
	}
	return vols[0].ID, nil
}

// AttachDisk attaches a Block Storage volume to a Droplet.
func (p *Provider) AttachDisk(ctx context.Context, vmName, diskName string) error {
	dropletID, err := p.getDropletIDByName(ctx, vmName)
	if err != nil {
		return err
	}

	volID, err := p.getVolumeIDByName(ctx, diskName)
	if err != nil {
		return err
	}

	action, _, err := p.client.StorageActions.Attach(ctx, volID, dropletID)
	if err != nil {
		return fmt.Errorf("failed to attach volume: %w", err)
	}

	// Wait for attachment to complete
	for i := 0; i < 60; i++ {
		a, _, err := p.client.StorageActions.Get(ctx, volID, action.ID)
		if err == nil && a.Status == "completed" {
			return nil
		}
		if err == nil && a.Status == "errored" {
			return fmt.Errorf("volume attachment errored")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	return fmt.Errorf("timeout waiting for volume to attach")
}

// DetachDisk detaches a Block Storage volume from a Droplet.
func (p *Provider) DetachDisk(ctx context.Context, vmName, diskName string) error {
	dropletID, err := p.getDropletIDByName(ctx, vmName)
	if err != nil {
		return err
	}

	volID, err := p.getVolumeIDByName(ctx, diskName)
	if err != nil {
		return err
	}

	action, _, err := p.client.StorageActions.DetachByDropletID(ctx, volID, dropletID)
	if err != nil {
		return fmt.Errorf("failed to detach volume: %w", err)
	}

	// Wait for detachment to complete
	for i := 0; i < 60; i++ {
		a, _, err := p.client.StorageActions.Get(ctx, volID, action.ID)
		if err == nil && a.Status == "completed" {
			return nil
		}
		if err == nil && a.Status == "errored" {
			return fmt.Errorf("volume detachment errored")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	return fmt.Errorf("timeout waiting for volume to detach")
}

// DeleteDisk permanently deletes a Block Storage volume.
func (p *Provider) DeleteDisk(ctx context.Context, diskName string) error {
	volID, err := p.getVolumeIDByName(ctx, diskName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}

	_, err = p.client.Storage.DeleteVolume(ctx, volID)
	if err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}

	return nil
}

// ValidateCredentials verifies the DIGITALOCEAN_TOKEN by fetching the account.
func (p *Provider) ValidateCredentials(ctx context.Context) error {
	if _, _, err := p.client.Account.Get(ctx); err != nil {
		return fmt.Errorf("digitalocean credentials check failed: %w", err)
	}
	return nil
}
