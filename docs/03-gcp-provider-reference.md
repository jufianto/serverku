# GCP Provider - Technical Reference

This document covers the specific GCP Compute Engine API calls needed for the serverku GCP provider implementation.

## Authentication

serverku uses Application Default Credentials (ADC). Users must run:
```bash
gcloud auth application-default login
```

In Go:
```go
import "google.golang.org/api/compute/v1"

service, err := compute.NewService(ctx)
// ADC is used automatically
```

Required IAM roles for the service account or user:
- `roles/compute.instanceAdmin.v1` - Create/delete VMs
- `roles/compute.storageAdmin` - Create/delete disks
- `roles/compute.securityAdmin` - Create firewall rules (or `roles/compute.networkAdmin`)

## API Calls Mapping

### CreateVM

```go
instance := &compute.Instance{
    Name:        "serverku-{project-name}",
    MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", zone, machineType),
    Tags:        &compute.Tags{Items: []string{"serverku", "serverku-{project-name}"}},
    Disks: []*compute.AttachedDisk{
        {
            AutoDelete: true,
            Boot:       true,
            InitializeParams: &compute.AttachedDiskInitializeParams{
                SourceImage: "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
                DiskSizeGb:  10,
                DiskType:    fmt.Sprintf("zones/%s/diskTypes/pd-standard", zone),
            },
        },
    },
    NetworkInterfaces: []*compute.NetworkInterface{
        {
            Network: "global/networks/default",
            AccessConfigs: []*compute.AccessConfig{
                {Type: "ONE_TO_ONE_NAT", Name: "External NAT"},
            },
        },
    },
    Scheduling: &compute.Scheduling{
        Preemptible:       spot,
        AutomaticRestart:  boolPtr(false),
        OnHostMaintenance: "TERMINATE",
        ProvisioningModel: "SPOT", // or "STANDARD"
    },
    Metadata: &compute.Metadata{
        Items: []*compute.MetadataItems{
            {Key: "ssh-keys", Value: stringPtr(fmt.Sprintf("serverku:%s", sshPubKey))},
        },
    },
}

op, err := service.Instances.Insert(projectID, zone, instance).Context(ctx).Do()
```

### DestroyVM

```go
op, err := service.Instances.Delete(projectID, zone, instanceName).Context(ctx).Do()
```

### CreateDisk

```go
disk := &compute.Disk{
    Name:   fmt.Sprintf("serverku-%s-data", projectName),
    SizeGb: sizeGB,
    Type:   fmt.Sprintf("zones/%s/diskTypes/pd-standard", zone),
}

op, err := service.Disks.Insert(projectID, zone, disk).Context(ctx).Do()
```

Disk pricing (pd-standard):
- $0.04/GB/month
- 20GB = $0.80/month when idle

### AttachDisk

```go
attachedDisk := &compute.AttachedDisk{
    Source:     fmt.Sprintf("zones/%s/disks/%s", zone, diskName),
    AutoDelete: false, // Important: don't delete disk when VM is deleted
}

op, err := service.Instances.AttachDisk(projectID, zone, instanceName, attachedDisk).Context(ctx).Do()
```

The disk will appear as `/dev/sdb` (or similar) inside the VM.

### DetachDisk

```go
op, err := service.Instances.DetachDisk(projectID, zone, instanceName, diskName).Context(ctx).Do()
```

### DeleteDisk

```go
op, err := service.Disks.Delete(projectID, zone, diskName).Context(ctx).Do()
```

### GetVMStatus

```go
instance, err := service.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
// instance.Status: "RUNNING", "TERMINATED", "STAGING", "STOPPING", etc.
```

### GetExternalIP

```go
instance, err := service.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
ip := instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
```

### WaitForReady

```go
// Poll until status is RUNNING
for {
    instance, err := service.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
    if instance.Status == "RUNNING" {
        return nil
    }
    time.Sleep(5 * time.Second)
}
```

Better approach - use operation polling:
```go
op, err := service.Instances.Insert(projectID, zone, instance).Context(ctx).Do()
// Wait for operation to complete
for op.Status != "DONE" {
    op, err = service.ZoneOperations.Get(projectID, zone, op.Name).Context(ctx).Do()
    time.Sleep(2 * time.Second)
}
```

### Firewall Rule

```go
firewall := &compute.Firewall{
    Name:    fmt.Sprintf("serverku-%s-fw", projectName),
    Network: "global/networks/default",
    Allowed: []*compute.FirewallAllowed{
        {IPProtocol: "tcp", Ports: []string{"22", "80", "443", "8080", "9000-9999"}},
        {IPProtocol: "icmp"},
    },
    TargetTags:   []string{fmt.Sprintf("serverku-%s", projectName)},
    SourceRanges: []string{"0.0.0.0/0"},
}

op, err := service.Firewalls.Insert(projectID, firewall).Context(ctx).Do()
```

## VM Sizing Reference

| Machine Type | vCPUs | Memory | SPOT Price/hr | Monthly (SPOT, 730h) |
|-------------|-------|--------|---------------|---------------------|
| e2-micro | 0.25 | 1GB | ~$0.003 | ~$2.19 |
| e2-small | 0.5 | 2GB | ~$0.007 | ~$5.11 |
| e2-medium | 1 | 4GB | ~$0.013 | ~$9.49 |
| e2-standard-2 | 2 | 8GB | ~$0.027 | ~$19.71 |
| e2-standard-4 | 4 | 16GB | ~$0.054 | ~$39.42 |

Note: SPOT prices are approximately 60-91% cheaper than on-demand. Actual prices vary by region.

## GCP-Specific Considerations

1. **SPOT termination**: GCP can reclaim SPOT VMs at any time. Since serverku VMs are ephemeral by design, this is acceptable. The user just runs `serverku up` again.

2. **Zone-locked disks**: Persistent disks exist in a specific zone. The VM must be created in the same zone as the disk.

3. **Default network**: We use the `default` VPC network for simplicity. Future: allow custom VPC.

4. **Boot disk vs data disk**: The boot disk (10GB, auto-delete) is separate from the data disk (persistent, user-configured size). Boot disk is always destroyed with the VM.

5. **Startup script**: We could use GCP metadata startup scripts, but SSH-based provisioning gives us more control and is provider-agnostic. The provisioner handles setup after the VM is RUNNING.

6. **Service account**: VMs are created without a service account by default. If the user's Docker containers need GCP API access, they should configure that in their project config (future feature).
