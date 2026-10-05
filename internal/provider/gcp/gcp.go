package gcp

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jufianto/serverku/internal/cloudlog"
	"github.com/jufianto/serverku/internal/provider"
	"google.golang.org/api/cloudbilling/v1"
	"google.golang.org/api/compute/v1"
	dns "google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
	htransport "google.golang.org/api/transport/http"
)

const (
	defaultBootDiskSizeGB = 10
	defaultBootDiskType   = "pd-standard"
	defaultDataDiskType   = "pd-standard"
	defaultImageFamily    = "ubuntu-2204-lts"
	defaultImageProject   = "ubuntu-os-cloud"
	defaultNetwork        = "global/networks/default"

	operationPollInterval = 3 * time.Second
	operationTimeout      = 10 * time.Minute
	readyPollInterval     = 5 * time.Second
	readyTimeout          = 5 * time.Minute
)

// GCPProvider implements provider.CloudProvider for Google Cloud Platform.
type GCPProvider struct {
	service        *compute.Service
	dnsService     *dns.Service
	billingService *cloudbilling.APIService
	projectID      string
	zone           string
}

// New creates a new GCPProvider using Application Default Credentials.
// The projectID and zone are used as defaults for all operations.
func New(ctx context.Context, projectID string, zone string) (*GCPProvider, error) {
	// Use an ADC-authenticated client shared by all cloud services. The
	// cloud-platform scope covers Compute, DNS and Billing; IAM still controls
	// the account's permissions. Wrap outside authentication so credentials
	// and token refresh traffic are never included in action logs.
	client, _, err := htransport.NewClient(ctx, option.WithScopes(compute.CloudPlatformScope))
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP HTTP client: %w", err)
	}
	client.Transport = &cloudlog.Transport{Provider: "gcp", Base: client.Transport}
	svc, err := compute.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP compute service: %w", err)
	}

	dnsSvc, err := dns.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP DNS service: %w", err)
	}

	// Billing catalog access is optional: without it, live pricing lookups
	// fail and callers fall back to labeled offline estimates.
	billingSvc, err := cloudbilling.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Printf("[gcp] cloud billing service unavailable (cost figures will be offline estimates): %v", err)
		billingSvc = nil
	}

	return &GCPProvider{
		service:        svc,
		dnsService:     dnsSvc,
		billingService: billingSvc,
		projectID:      projectID,
		zone:           zone,
	}, nil
}

// NewWithBillingService creates a GCPProvider with an injected billing
// service (for testing pricing lookups).
func NewWithBillingService(billingSvc *cloudbilling.APIService, projectID string, zone string) *GCPProvider {
	return &GCPProvider{
		billingService: billingSvc,
		projectID:      projectID,
		zone:           zone,
	}
}

// NewWithService creates a GCPProvider with an injected compute service (for testing).
func NewWithService(svc *compute.Service, projectID string, zone string) *GCPProvider {
	return &GCPProvider{
		service:   svc,
		projectID: projectID,
		zone:      zone,
	}
}

// NewWithServices creates a GCPProvider with injected compute and DNS services
// (for testing DNS-related operations).
func NewWithServices(svc *compute.Service, dnsSvc *dns.Service, projectID string, zone string) *GCPProvider {
	return &GCPProvider{
		service:    svc,
		dnsService: dnsSvc,
		projectID:  projectID,
		zone:       zone,
	}
}

func (g *GCPProvider) resolveZone(zone string) string {
	if zone != "" {
		return zone
	}
	return g.zone
}

func (g *GCPProvider) resolveProjectID(projectID string) string {
	if projectID != "" {
		return projectID
	}
	return g.projectID
}

// CreateVM creates a new GCP Compute Engine instance.
func (g *GCPProvider) CreateVM(ctx context.Context, config provider.VMConfig) (*provider.VM, error) {
	zone := g.resolveZone(config.Zone)
	projectID := g.resolveProjectID(config.ProjectID)

	// Resolve image
	imageURL := fmt.Sprintf("projects/%s/global/images/family/%s", defaultImageProject, resolveImageFamily(config.Image))

	// Build scheduling config
	scheduling := &compute.Scheduling{
		OnHostMaintenance: "TERMINATE",
	}
	if config.Spot {
		scheduling.Preemptible = true
		scheduling.AutomaticRestart = boolPtr(false)
		scheduling.ProvisioningModel = "SPOT"
	} else {
		scheduling.AutomaticRestart = boolPtr(true)
		scheduling.ProvisioningModel = "STANDARD"
	}

	// max_uptime: let GCP itself delete the instance after N hours as a hard
	// budget cap. DELETE (not STOP) matches serverku's disposable-VM model and
	// actually stops billing; the non-boot data disk is not auto-delete, so it
	// survives and reattaches on the next `up`. status reconciles the gone VM.
	if config.MaxUptimeHours > 0 {
		scheduling.MaxRunDuration = &compute.Duration{Seconds: int64(config.MaxUptimeHours) * 3600}
		scheduling.InstanceTerminationAction = "DELETE"
		// maxRunDuration requires automaticRestart to be false.
		scheduling.AutomaticRestart = boolPtr(false)
	}

	// Build instance tags
	tags := config.Tags
	if len(tags) == 0 {
		tags = []string{"serverku", fmt.Sprintf("serverku-%s", config.Name)}
	}

	// Build metadata for SSH key injection
	var metadataItems []*compute.MetadataItems
	if config.SSHPubKey != "" {
		sshEntry := fmt.Sprintf("serverku:%s", strings.TrimSpace(config.SSHPubKey))
		metadataItems = append(metadataItems, &compute.MetadataItems{
			Key:   "ssh-keys",
			Value: stringPtr(sshEntry),
		})
	}
	if config.StartupScript != "" {
		metadataItems = append(metadataItems, &compute.MetadataItems{
			Key:   "startup-script",
			Value: stringPtr(config.StartupScript),
		})
	}

	instance := &compute.Instance{
		Name:        config.Name,
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", zone, config.MachineType),
		Tags:        &compute.Tags{Items: tags},
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					SourceImage: imageURL,
					DiskSizeGb:  defaultBootDiskSizeGB,
					DiskType:    fmt.Sprintf("zones/%s/diskTypes/%s", zone, defaultBootDiskType),
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network: defaultNetwork,
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
			},
		},
		Scheduling: scheduling,
		Metadata: &compute.Metadata{
			Items: metadataItems,
		},
	}

	if config.SSHPubKey != "" {
		log.Printf("[gcp] injecting SSH public key for serverku user via instance metadata")
	}
	log.Printf("[gcp] creating VM %q in zone %s (type: %s, spot: %v)", config.Name, zone, config.MachineType, config.Spot)

	op, err := g.service.Instances.Insert(projectID, zone, instance).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create VM %q: %w", config.Name, err)
	}

	if err := g.waitForZoneOperation(ctx, projectID, zone, op.Name); err != nil {
		return nil, fmt.Errorf("failed waiting for VM creation: %w", err)
	}

	// Get the created instance to retrieve its ID
	created, err := g.service.Instances.Get(projectID, zone, config.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("VM created but failed to retrieve details: %w", err)
	}

	log.Printf("[gcp] VM %q created (id: %d)", config.Name, created.Id)

	return &provider.VM{
		ID:       strconv.FormatUint(created.Id, 10),
		Name:     created.Name,
		Zone:     zone,
		Provider: "gcp",
	}, nil
}

// DestroyVM deletes a GCP Compute Engine instance by name.
func (g *GCPProvider) DestroyVM(ctx context.Context, vmID string) error {
	// vmID is used as the instance name for GCP
	log.Printf("[gcp] destroying VM %q", vmID)

	op, err := g.service.Instances.Delete(g.projectID, g.zone, vmID).Context(ctx).Do()
	if err != nil {
		if isNotFoundErr(err) {
			log.Printf("[gcp] VM %q already deleted", vmID)
			return nil
		}
		return fmt.Errorf("failed to destroy VM %q: %w", vmID, err)
	}

	return g.waitForZoneOperation(ctx, g.projectID, g.zone, op.Name)
}

// StartVM starts a stopped GCP instance.
func (g *GCPProvider) StartVM(ctx context.Context, vmID string) error {
	log.Printf("[gcp] starting VM %q", vmID)

	op, err := g.service.Instances.Start(g.projectID, g.zone, vmID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to start VM %q: %w", vmID, err)
	}

	return g.waitForZoneOperation(ctx, g.projectID, g.zone, op.Name)
}

// StopVM stops a running GCP instance.
func (g *GCPProvider) StopVM(ctx context.Context, vmID string) error {
	log.Printf("[gcp] stopping VM %q", vmID)

	op, err := g.service.Instances.Stop(g.projectID, g.zone, vmID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to stop VM %q: %w", vmID, err)
	}

	return g.waitForZoneOperation(ctx, g.projectID, g.zone, op.Name)
}

// GetVMStatus returns the current status of a GCP instance.
func (g *GCPProvider) GetVMStatus(ctx context.Context, vmID string) (*provider.VMStatus, error) {
	instance, err := g.service.Instances.Get(g.projectID, g.zone, vmID).Context(ctx).Do()
	if err != nil {
		if isNotFoundErr(err) {
			return &provider.VMStatus{
				Name:  vmID,
				State: provider.VMStateTerminated,
			}, nil
		}
		return nil, fmt.Errorf("failed to get VM status: %w", err)
	}

	return &provider.VMStatus{
		ID:         strconv.FormatUint(instance.Id, 10),
		Name:       instance.Name,
		State:      mapGCPStatus(instance.Status),
		ExternalIP: extractExternalIP(instance),
	}, nil
}

// GetExternalIP returns the external IP address of a running VM.
func (g *GCPProvider) GetExternalIP(ctx context.Context, vmID string) (string, error) {
	instance, err := g.service.Instances.Get(g.projectID, g.zone, vmID).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("failed to get VM: %w", err)
	}

	ip := extractExternalIP(instance)
	if ip == "" {
		return "", fmt.Errorf("VM %q has no external IP", vmID)
	}

	return ip, nil
}

// WaitForReady polls until the VM is in RUNNING state.
func (g *GCPProvider) WaitForReady(ctx context.Context, vmID string) error {
	log.Printf("[gcp] waiting for VM %q to be ready...", vmID)

	deadline := time.Now().Add(readyTimeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for VM %q to become ready", vmID)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		instance, err := g.service.Instances.Get(g.projectID, g.zone, vmID).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to poll VM status: %w", err)
		}

		if instance.Status == "RUNNING" {
			log.Printf("[gcp] VM %q is ready", vmID)
			return nil
		}

		log.Printf("[gcp] VM %q status: %s, waiting...", vmID, instance.Status)
		time.Sleep(readyPollInterval)
	}
}

// CreateDisk creates a new persistent disk.
func (g *GCPProvider) CreateDisk(ctx context.Context, config provider.DiskConfig) (*provider.Disk, error) {
	zone := g.resolveZone(config.Zone)
	projectID := g.resolveProjectID(config.ProjectID)

	diskType := config.DiskType
	if diskType == "" {
		diskType = defaultDataDiskType
	}

	disk := &compute.Disk{
		Name:   config.Name,
		SizeGb: config.SizeGB,
		Type:   fmt.Sprintf("zones/%s/diskTypes/%s", zone, diskType),
	}

	log.Printf("[gcp] creating disk %q (%dGB, type: %s) in zone %s", config.Name, config.SizeGB, diskType, zone)

	op, err := g.service.Disks.Insert(projectID, zone, disk).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create disk %q: %w", config.Name, err)
	}

	if err := g.waitForZoneOperation(ctx, projectID, zone, op.Name); err != nil {
		return nil, fmt.Errorf("failed waiting for disk creation: %w", err)
	}

	created, err := g.service.Disks.Get(projectID, zone, config.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("disk created but failed to retrieve details: %w", err)
	}

	log.Printf("[gcp] disk %q created (id: %d)", config.Name, created.Id)

	return &provider.Disk{
		ID:       strconv.FormatUint(created.Id, 10),
		Name:     created.Name,
		Zone:     zone,
		SizeGB:   created.SizeGb,
		Provider: "gcp",
	}, nil
}

// DeleteDisk permanently deletes a persistent disk.
func (g *GCPProvider) DeleteDisk(ctx context.Context, diskID string) error {
	log.Printf("[gcp] deleting disk %q", diskID)

	op, err := g.service.Disks.Delete(g.projectID, g.zone, diskID).Context(ctx).Do()
	if err != nil {
		if isNotFoundErr(err) {
			log.Printf("[gcp] disk %q already deleted", diskID)
			return nil
		}
		return fmt.Errorf("failed to delete disk %q: %w", diskID, err)
	}

	return g.waitForZoneOperation(ctx, g.projectID, g.zone, op.Name)
}

// AttachDisk attaches a persistent disk to a running VM.
func (g *GCPProvider) AttachDisk(ctx context.Context, vmID string, diskID string) error {
	log.Printf("[gcp] attaching disk %q to VM %q", diskID, vmID)

	attachedDisk := &compute.AttachedDisk{
		Source:     fmt.Sprintf("zones/%s/disks/%s", g.zone, diskID),
		AutoDelete: false, // Critical: don't delete disk when VM is deleted
	}

	op, err := g.service.Instances.AttachDisk(g.projectID, g.zone, vmID, attachedDisk).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to attach disk %q to VM %q: %w", diskID, vmID, err)
	}

	return g.waitForZoneOperation(ctx, g.projectID, g.zone, op.Name)
}

// DetachDisk detaches a persistent disk from a VM.
func (g *GCPProvider) DetachDisk(ctx context.Context, vmID string, diskID string) error {
	log.Printf("[gcp] detaching disk %q from VM %q", diskID, vmID)

	op, err := g.service.Instances.DetachDisk(g.projectID, g.zone, vmID, diskID).Context(ctx).Do()
	if err != nil {
		if isNotFoundErr(err) {
			log.Printf("[gcp] disk %q already detached", diskID)
			return nil
		}
		return fmt.Errorf("failed to detach disk %q from VM %q: %w", diskID, vmID, err)
	}

	return g.waitForZoneOperation(ctx, g.projectID, g.zone, op.Name)
}

// EnsureFirewall creates a firewall rule for the project if it doesn't already exist.
func (g *GCPProvider) EnsureFirewall(ctx context.Context, projectName string) error {
	fwName := fmt.Sprintf("serverku-%s-fw", projectName)

	// Check if firewall already exists
	_, err := g.service.Firewalls.Get(g.projectID, fwName).Context(ctx).Do()
	if err == nil {
		log.Printf("[gcp] firewall %q already exists", fwName)
		return nil
	}
	if !isNotFoundErr(err) {
		return fmt.Errorf("failed to check firewall: %w", err)
	}

	firewall := &compute.Firewall{
		Name:    fwName,
		Network: defaultNetwork,
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports:      []string{"22", "80", "443", "8080", "9000-9999"},
			},
			{
				IPProtocol: "icmp",
			},
		},
		TargetTags:   []string{fmt.Sprintf("serverku-%s", projectName)},
		SourceRanges: []string{"0.0.0.0/0"},
	}

	log.Printf("[gcp] creating firewall rule %q", fwName)

	op, err := g.service.Firewalls.Insert(g.projectID, firewall).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to create firewall rule: %w", err)
	}

	return g.waitForGlobalOperation(ctx, g.projectID, op.Name)
}

// DeleteFirewall removes the firewall rule for a project.
func (g *GCPProvider) DeleteFirewall(ctx context.Context, projectName string) error {
	fwName := fmt.Sprintf("serverku-%s-fw", projectName)

	log.Printf("[gcp] deleting firewall rule %q", fwName)

	op, err := g.service.Firewalls.Delete(g.projectID, fwName).Context(ctx).Do()
	if err != nil {
		if isNotFoundErr(err) {
			return nil
		}
		return fmt.Errorf("failed to delete firewall rule: %w", err)
	}

	return g.waitForGlobalOperation(ctx, g.projectID, op.Name)
}

// waitForZoneOperation polls a zone-scoped operation until it completes.
func (g *GCPProvider) waitForZoneOperation(ctx context.Context, projectID, zone, opName string) error {
	deadline := time.Now().Add(operationTimeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for operation %q", opName)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		op, err := g.service.ZoneOperations.Get(projectID, zone, opName).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to poll operation %q: %w", opName, err)
		}

		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("operation %q failed: %s", opName, op.Error.Errors[0].Message)
			}
			return nil
		}

		time.Sleep(operationPollInterval)
	}
}

// waitForGlobalOperation polls a global-scoped operation until it completes.
func (g *GCPProvider) waitForGlobalOperation(ctx context.Context, projectID, opName string) error {
	deadline := time.Now().Add(operationTimeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for operation %q", opName)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		op, err := g.service.GlobalOperations.Get(projectID, opName).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to poll operation %q: %w", opName, err)
		}

		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("operation %q failed: %s", opName, op.Error.Errors[0].Message)
			}
			return nil
		}

		time.Sleep(operationPollInterval)
	}
}

// mapGCPStatus converts a GCP instance status string to a VMStatusState.
func mapGCPStatus(status string) provider.VMStatusState {
	switch status {
	case "RUNNING":
		return provider.VMStateRunning
	case "STOPPED", "SUSPENDED":
		return provider.VMStateStopped
	case "STAGING", "PROVISIONING":
		return provider.VMStateStarting
	case "STOPPING", "SUSPENDING":
		return provider.VMStateStopping
	case "TERMINATED":
		return provider.VMStateTerminated
	default:
		return provider.VMStateUnknown
	}
}

// extractExternalIP extracts the external IP from a GCP instance.
func extractExternalIP(instance *compute.Instance) string {
	if instance == nil {
		return ""
	}
	for _, ni := range instance.NetworkInterfaces {
		for _, ac := range ni.AccessConfigs {
			if ac.NatIP != "" {
				return ac.NatIP
			}
		}
	}
	return ""
}

// resolveImageFamily maps a short image name to a GCP image family.
func resolveImageFamily(image string) string {
	switch image {
	case "ubuntu-22-04", "ubuntu-2204", "":
		return defaultImageFamily
	case "ubuntu-20-04", "ubuntu-2004":
		return "ubuntu-2004-lts"
	case "ubuntu-24-04", "ubuntu-2404":
		return "ubuntu-2404-lts-amd64"
	default:
		return image
	}
}

// isNotFoundErr checks if the error is a 404 not found error.
func isNotFoundErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "notFound")
}

func boolPtr(b bool) *bool       { return &b }
func stringPtr(s string) *string { return &s }

// ValidateCredentials verifies Application Default Credentials and project
// access with a cheap regions list (capped to one result).
func (g *GCPProvider) ValidateCredentials(ctx context.Context) error {
	if _, err := g.service.Regions.List(g.projectID).MaxResults(1).Context(ctx).Do(); err != nil {
		return fmt.Errorf("gcp credentials check failed: %w", err)
	}
	return nil
}

// ListComponents reports the live state of the GCP resources serverku manages
// for a project: the instance, disk and firewall rule (all removed by destroy),
// plus any disk snapshots (orphans -- destroy leaves them). The SSH key rides
// in instance metadata and dies with the VM, so it is not a separate resource.
func (g *GCPProvider) ListComponents(ctx context.Context, q provider.ComponentQuery) ([]provider.Component, error) {
	var comps []provider.Component

	// VM instance.
	vmPresent := false
	if _, err := g.service.Instances.Get(g.projectID, g.zone, q.VMName).Context(ctx).Do(); err == nil {
		vmPresent = true
	}
	comps = append(comps, provider.Component{Kind: "VM", Name: q.VMName, Present: vmPresent, RemovedByDestroy: true})

	// Persistent disk.
	if q.DiskName != "" {
		present, detail := false, ""
		if d, err := g.service.Disks.Get(g.projectID, g.zone, q.DiskName).Context(ctx).Do(); err == nil {
			present = true
			detail = fmt.Sprintf("%dGB", d.SizeGb)
		}
		comps = append(comps, provider.Component{Kind: "Volume", Name: q.DiskName, Detail: detail, Present: present, RemovedByDestroy: true})
	}

	// Firewall rule.
	fwName := fmt.Sprintf("serverku-%s-fw", q.ProjectName)
	fwPresent := false
	if _, err := g.service.Firewalls.Get(g.projectID, fwName).Context(ctx).Do(); err == nil {
		fwPresent = true
	}
	comps = append(comps, provider.Component{Kind: "Firewall", Name: fwName, Present: fwPresent, RemovedByDestroy: true})

	// Disk snapshots -- orphan. Match by source disk name suffix.
	if q.DiskName != "" {
		if list, err := g.service.Snapshots.List(g.projectID).Context(ctx).Do(); err == nil {
			count := 0
			for _, s := range list.Items {
				if strings.HasSuffix(s.SourceDisk, "/"+q.DiskName) {
					count++
				}
			}
			comps = append(comps, provider.Component{Kind: "Snapshot", Detail: fmt.Sprintf("%d found", count), Present: count > 0, RemovedByDestroy: false})
		}
	}

	return comps, nil
}
