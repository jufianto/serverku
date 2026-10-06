package digitalocean

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/jufianto/serverku/internal/cloudlog"
	"github.com/jufianto/serverku/internal/provider"
	"golang.org/x/crypto/ssh"
)

// Provider implements the CloudProvider interface for DigitalOcean.
type Provider struct {
	client *godo.Client
}

// New creates a new DigitalOcean provider instance from the
// DIGITALOCEAN_TOKEN environment variable.
func New(ctx context.Context) (*Provider, error) {
	token := os.Getenv("DIGITALOCEAN_TOKEN")
	if token == "" {
		return nil, errors.New("DIGITALOCEAN_TOKEN environment variable is not set")
	}
	return NewWithToken(token)
}

// NewWithToken creates a new DigitalOcean provider instance from an explicit
// API token. Callers resolve the token however they like (env var, saved
// credentials file) and pass it here.
func NewWithToken(token string) (*Provider, error) {
	if token == "" {
		return nil, errors.New("digitalocean token is empty")
	}
	client := godo.NewFromToken(token)
	client.HTTPClient.Transport = &cloudlog.Transport{Provider: "digitalocean", Base: client.HTTPClient.Transport}
	return &Provider{client: client}, nil
}

// CreateVM creates a new Droplet.
func (p *Provider) CreateVM(ctx context.Context, cfg provider.VMConfig) (*provider.VM, error) {
	if cfg.Spot {
		return nil, errors.New("DigitalOcean provider does not support spot instances")
	}

	key := &provider.SSHKey{ID: cfg.SSHKeyID}
	if key.ID == "" {
		var err error
		key, err = p.ensureSSHKey(ctx, "serverku-"+cfg.Name, cfg.SSHPubKey)
		if err != nil {
			return nil, err
		}
	}
	keyID, err := strconv.Atoi(key.ID)
	if err != nil || keyID <= 0 {
		return nil, fmt.Errorf("invalid DigitalOcean SSH key ID %q", key.ID)
	}

	// Older init versions wrote GCP's image family for DigitalOcean too.
	// Translate that exact legacy default to the corresponding DO slug.
	image := cfg.Image
	if image == "" || image == "ubuntu-22-04" {
		image = "ubuntu-22-04-x64"
	}

	createRequest := &godo.DropletCreateRequest{
		Name:   cfg.Name,
		Region: cfg.Region,
		Size:   cfg.MachineType,
		Image: godo.DropletCreateImage{
			Slug: image,
		},
		SSHKeys: []godo.DropletCreateSSHKey{
			{ID: keyID, Fingerprint: key.Fingerprint},
		},
		Tags: cfg.Tags,
		// DigitalOcean injects the account SSH key into root's authorized_keys
		// only; it has no equivalent of GCP's guest-agent user creation. serverku
		// connects as the "serverku" user (see orchestrator), so create that user
		// via cloud-init with the same key and passwordless sudo.
		UserData: serverkuUserData(cfg.SSHPubKey),
	}

	log.Printf("[digitalocean] creating droplet %q in %s (size: %s, image: %s); injecting SSH key and configuring serverku user via cloud-init", cfg.Name, cfg.Region, cfg.MachineType, image)
	droplet, _, err := p.client.Droplets.Create(ctx, createRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create droplet: %w", err)
	}

	log.Printf("[digitalocean] droplet %q created (id: %d)", droplet.Name, droplet.ID)
	return &provider.VM{
		ID:       fmt.Sprintf("%d", droplet.ID),
		Name:     droplet.Name,
		Zone:     cfg.Region,
		Provider: "digitalocean",
	}, nil
}

// ensureSSHKey reuses the account key by identity, regardless of its name.
// Multiple projects share the local serverku key; DigitalOcean rejects duplicate
// public keys even when they are registered under different names.
func (p *Provider) ensureSSHKey(ctx context.Context, name, publicKey string) (*provider.SSHKey, error) {
	parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid SSH public key: %w", err)
	}
	fingerprint := ssh.FingerprintLegacyMD5(parsed)
	log.Printf("[digitalocean] looking up account SSH key by fingerprint")
	key, response, err := p.client.Keys.GetByFingerprint(ctx, fingerprint)
	if err == nil {
		log.Printf("[digitalocean] reusing SSH key %q (id: %d)", key.Name, key.ID)
		return accountSSHKey(key, false), nil
	}
	if response == nil || response.StatusCode != http.StatusNotFound {
		return nil, fmt.Errorf("failed to look up SSH key in DigitalOcean: %w", err)
	}

	log.Printf("[digitalocean] registering SSH key %q", name)
	key, response, err = p.client.Keys.Create(ctx, &godo.KeyCreateRequest{
		Name: name, PublicKey: strings.TrimSpace(publicKey),
	})
	if err == nil {
		log.Printf("[digitalocean] SSH key %q registered (id: %d)", key.Name, key.ID)
		return accountSSHKey(key, true), nil
	}
	// Another concurrent project may have registered the same key after our
	// lookup. Only accept the conflict if that exact fingerprint now exists.
	if response != nil && response.StatusCode == http.StatusUnprocessableEntity {
		if existing, _, lookupErr := p.client.Keys.GetByFingerprint(ctx, fingerprint); lookupErr == nil {
			log.Printf("[digitalocean] reusing concurrently registered SSH key %q (id: %d)", existing.Name, existing.ID)
			return accountSSHKey(existing, false), nil
		}
	}
	return nil, fmt.Errorf("failed to create SSH key in DigitalOcean: %w", err)
}

func accountSSHKey(key *godo.Key, created bool) *provider.SSHKey {
	return &provider.SSHKey{ID: strconv.Itoa(key.ID), Name: key.Name, Fingerprint: key.Fingerprint, Created: created}
}

func (p *Provider) EnsureProjectSSHKey(ctx context.Context, projectName, publicKey string) (*provider.SSHKey, error) {
	return p.ensureSSHKey(ctx, "serverku-"+projectName, publicKey)
}

// DeleteSSHKey verifies both ID and fingerprint before deleting an owned key.
func (p *Provider) DeleteSSHKey(ctx context.Context, id, publicKey string) error {
	keyID, err := strconv.Atoi(id)
	if err != nil || keyID <= 0 {
		return fmt.Errorf("invalid SSH key ID %q", id)
	}
	parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return fmt.Errorf("invalid recorded SSH public key: %w", err)
	}
	key, response, err := p.client.Keys.GetByID(ctx, keyID)
	if response != nil && response.StatusCode == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to verify SSH key before deletion: %w", err)
	}
	if key.Fingerprint != ssh.FingerprintLegacyMD5(parsed) {
		return fmt.Errorf("SSH key %s fingerprint differs from recorded key; refusing deletion", id)
	}
	log.Printf("[digitalocean] deleting project SSH key %q (id: %s)", key.Name, id)
	response, err = p.client.Keys.DeleteByID(ctx, keyID)
	if response != nil && response.StatusCode == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete SSH key: %w", err)
	}
	return nil
}

// serverkuUserData returns a cloud-init config that provisions the "serverku"
// login user with passwordless sudo and the given public key. Returns an empty
// string when no key is provided so the droplet keeps DigitalOcean's default
// (root-only) SSH setup.
func serverkuUserData(pubKey string) string {
	pubKey = strings.TrimSpace(pubKey)
	if pubKey == "" {
		return ""
	}
	return fmt.Sprintf(`#cloud-config
users:
  - name: serverku
    groups: sudo
    sudo: ['ALL=(ALL) NOPASSWD:ALL']
    shell: /bin/bash
    ssh_authorized_keys:
      - %s
`, pubKey)
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
	log.Printf("[digitalocean] powering off droplet %q", name)
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
	log.Printf("[digitalocean] powering on droplet %q", name)
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
	log.Printf("[digitalocean] deleting droplet %q", name)
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
	log.Printf("[digitalocean] waiting for droplet to become active %q", name)
	maxRetries := 60 // 60 * 5s = 5 minutes
	for i := 0; i < maxRetries; i++ {
		status, err := p.GetVMStatus(ctx, name)
		if err != nil {
			log.Printf("[digitalocean] error checking droplet status: %v", err)
		} else if status.State == provider.VMStateRunning {
			log.Printf("[digitalocean] droplet %q is active; SSH readiness is checked during provisioning", name)
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
	log.Printf("[digitalocean] creating volume %q (%d GB) in %s", cfg.Name, cfg.SizeGB, cfg.Zone)
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

	log.Printf("[digitalocean] volume %q created (id: %s)", vol.Name, vol.ID)
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
	log.Printf("[digitalocean] attaching volume %q for droplet %q", diskName, vmName)
	dropletID, err := p.getDropletIDByName(ctx, vmName)
	if err != nil {
		return err
	}

	volID, err := p.getVolumeIDByName(ctx, diskName)
	if err != nil {
		return err
	}

	return p.attachVolume(ctx, dropletID, volID)
}

// AttachDiskByID uses the UUID returned by volume creation (or saved in state).
// A newly created volume may not yet appear in the name-filtered listing.
func (p *Provider) AttachDiskByID(ctx context.Context, vmName, diskID string) error {
	if diskID == "" {
		return errors.New("volume ID is empty")
	}
	log.Printf("[digitalocean] attaching volume ID %q for droplet %q", diskID, vmName)
	dropletID, err := p.getDropletIDByName(ctx, vmName)
	if err != nil {
		return err
	}
	return p.attachVolume(ctx, dropletID, diskID)
}

func (p *Provider) attachVolume(ctx context.Context, dropletID int, volID string) error {
	action, _, err := p.client.StorageActions.Attach(ctx, volID, dropletID)
	if err != nil {
		return fmt.Errorf("failed to attach volume %q: %w", volID, err)
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
	log.Printf("[digitalocean] detaching volume %q for droplet %q", diskName, vmName)
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
	log.Printf("[digitalocean] deleting volume %q", diskName)
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

// AccountEmail fetches the account the token belongs to and returns its email.
// It doubles as a credential check (a bad token fails here) and lets
// `serverku setup` show which account is being configured.
func (p *Provider) AccountEmail(ctx context.Context) (string, error) {
	acct, _, err := p.client.Account.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("digitalocean credentials check failed: %w", err)
	}
	return acct.Email, nil
}

// ListComponents reports the live state of the DigitalOcean resources serverku
// manages for a project: the droplet and volume (removed by destroy), plus the
// shared account SSH key (intentionally retained) and volume snapshots
// (orphans -- destroy leaves them).
func (p *Provider) ListComponents(ctx context.Context, q provider.ComponentQuery) ([]provider.Component, error) {
	var comps []provider.Component

	// VM (droplet).
	vmPresent := false
	if _, err := p.GetVM(ctx, q.VMName); err == nil {
		vmPresent = true
	}
	comps = append(comps, provider.Component{Kind: "VM", Name: q.VMName, Present: vmPresent, RemovedByDestroy: true})

	// Volume.
	var volID string
	if q.DiskName != "" {
		present, detail := false, ""
		if vols, _, err := p.client.Storage.ListVolumes(ctx, &godo.ListVolumeParams{Name: q.DiskName}); err == nil && len(vols) > 0 {
			present, volID = true, vols[0].ID
			detail = fmt.Sprintf("%dGB", int(vols[0].SizeGigaBytes))
		}
		comps = append(comps, provider.Component{Kind: "Volume", Name: q.DiskName, Detail: detail, Present: present, RemovedByDestroy: true})
	}

	// All local projects share a key. CreateVM reuses it by fingerprint,
	// regardless of which project originally registered its account name.
	keyComponent := provider.Component{Kind: "SSH key", Name: "local serverku key", Shared: !q.SSHKeyOwned, RemovedByDestroy: q.SSHKeyOwned}
	if q.SSHPubKey == "" {
		keyComponent.LookupFailed = true
		keyComponent.Detail = "local public key unavailable"
	} else if parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(q.SSHPubKey)); err != nil {
		keyComponent.LookupFailed = true
		keyComponent.Detail = "invalid local public key"
	} else {
		key, response, err := p.client.Keys.GetByFingerprint(ctx, ssh.FingerprintLegacyMD5(parsed))
		switch {
		case err == nil:
			keyComponent.Name = key.Name
			keyComponent.Present = true
			keyComponent.Detail = fmt.Sprintf("id: %d", key.ID)
		case response != nil && response.StatusCode == http.StatusNotFound:
			keyComponent.Detail = "not registered"
		default:
			keyComponent.LookupFailed = true
			keyComponent.Detail = "account key lookup failed"
		}
	}
	comps = append(comps, keyComponent)

	// Volume snapshots -- orphan. Attributable while the volume exists.
	if volID != "" {
		if snaps, _, err := p.client.Snapshots.ListVolume(ctx, &godo.ListOptions{PerPage: 200}); err == nil {
			count := 0
			for _, s := range snaps {
				if s.ResourceID == volID {
					count++
				}
			}
			comps = append(comps, provider.Component{Kind: "Snapshot", Detail: fmt.Sprintf("%d found", count), Present: count > 0, RemovedByDestroy: false})
		}
	}

	return comps, nil
}
