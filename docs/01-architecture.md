# Architecture Overview

## System Design

serverku is a CLI tool that orchestrates cloud VMs on demand. The system is composed of four core components:

### 1. CLI Layer (`cmd/serverku/`)

The user-facing command-line interface built with cobra. Handles argument parsing, user prompts, and output formatting. Delegates all logic to the orchestrator.

Commands:
- `init` - Interactive project setup, writes YAML config
- `up` - Full lifecycle: create VM -> attach disk -> provision -> deploy
- `down` - Reverse lifecycle: stop containers -> detach disk -> destroy VM
- `status` - Query VM state and display info
- `list` - Enumerate all projects from config directory
- `destroy` - Permanent deletion of all resources including storage

### 2. Orchestrator (`internal/orchestrator/`)

The coordination layer. Receives commands from the CLI and orchestrates the correct sequence of provider and provisioner calls. This is where the core business logic lives.

**Up sequence:**
```
1. Load project config from store
2. Load project state (check if already running)
3. If storage.enabled and no disk exists:
     -> provider.CreateDisk()
     -> save disk ID to state
4. provider.CreateVM() with spot/preemptible config
     -> save VM ID to state
5. If storage.enabled:
     -> provider.AttachDisk(vmID, diskID)
6. provider.WaitForReady(vmID)
7. provider.GetExternalIP(vmID)
     -> save IP to state
8. provisioner.Provision(ip, sshKey, config)
     -> install Docker
     -> mount disk (if storage enabled)
     -> copy/pull docker-compose
     -> docker compose up -d
9. Update state: status = running, started_at = now
10. If notify configured:
      -> telegram.Send("Project X is up at IP Y")
```

**Down sequence:**
```
1. Load project config and state
2. Verify VM is running
3. provisioner.Teardown(ip, sshKey)
     -> docker compose down
     -> unmount disk
4. If storage.enabled:
     -> provider.DetachDisk(vmID, diskID)
5. provider.DestroyVM(vmID)
6. Update state: status = stopped, vm_id = "", ip = ""
7. If notify configured:
      -> telegram.Send("Project X is down")
```

### 3. Provider Layer (`internal/provider/`)

Abstract interface for cloud operations. Each cloud provider implements the same interface, making the orchestrator cloud-agnostic.

```go
type CloudProvider interface {
    // VM lifecycle
    CreateVM(ctx context.Context, config VMConfig) (*VM, error)
    DestroyVM(ctx context.Context, vmID string) error
    StartVM(ctx context.Context, vmID string) error
    StopVM(ctx context.Context, vmID string) error
    GetVMStatus(ctx context.Context, vmID string) (*VMStatus, error)
    GetExternalIP(ctx context.Context, vmID string) (string, error)
    WaitForReady(ctx context.Context, vmID string) error

    // Disk lifecycle
    CreateDisk(ctx context.Context, config DiskConfig) (*Disk, error)
    DeleteDisk(ctx context.Context, diskID string) error
    AttachDisk(ctx context.Context, vmID string, diskID string) error
    DetachDisk(ctx context.Context, vmID string, diskID string) error
}
```

**VMConfig** is provider-agnostic:
```go
type VMConfig struct {
    Name       string
    Region     string
    Zone       string
    MachineType string   // e.g., "e2-medium" (GCP), "s-1vcpu-1gb" (DO)
    Image      string   // e.g., "ubuntu-22-04"
    Spot       bool
    Tags       []string
    SSHPubKey  string
    StartupScript string
}
```

**Provider mapping:**

| VMConfig field | GCP | DigitalOcean |
|---------------|-----|-------------|
| MachineType | `machineType` on Compute Engine | `Size` slug on Droplet |
| Image | `sourceImage` on boot disk | `Image` slug on Droplet |
| Spot | `scheduling.preemptible = true` | Not available (use reserved instead) |
| Region/Zone | `zone` (e.g., `asia-southeast1-b`) | `region` (e.g., `sgp1`) |
| Disk | Persistent Disk resource | Volume resource |

### 4. Provisioner (`internal/provisioner/`)

SSH-based server setup. After the VM is created and accessible, the provisioner connects via SSH and runs the setup sequence.

**Provisioning steps:**
```
1. Wait for SSH to be available (retry with backoff)
2. Install Docker and Docker Compose
3. If storage.enabled:
     a. Format disk if new (mkfs.ext4)
     b. Mount disk to mount_path (e.g., /data)
     c. Add to /etc/fstab for persistence
4. Transfer docker-compose.yml to VM (scp or inline)
5. Run: docker compose up -d
6. Verify containers are running
```

**SSH key management:**
- serverku generates an SSH key pair on first run
- Stored at `~/.serverku/keys/serverku_rsa` and `serverku_rsa.pub`
- Public key is injected into VM metadata during creation
- Used by provisioner to connect

## Data Flow

```
User                CLI                 Orchestrator        Provider         Provisioner
 |                   |                      |                  |                 |
 |-- serverku up --> |                      |                  |                 |
 |                   |-- Up(projectName) -> |                  |                 |
 |                   |                      |-- CreateDisk --> |                 |
 |                   |                      |<-- diskID -------|                 |
 |                   |                      |-- CreateVM ----> |                 |
 |                   |                      |<-- vmID ---------|                 |
 |                   |                      |-- AttachDisk --> |                 |
 |                   |                      |-- WaitReady ---> |                 |
 |                   |                      |-- GetIP -------> |                 |
 |                   |                      |<-- ip -----------|                 |
 |                   |                      |-- Provision(ip) ----------------> |
 |                   |                      |                                   |-- SSH
 |                   |                      |                                   |-- Docker
 |                   |                      |                                   |-- Mount
 |                   |                      |                                   |-- Compose
 |                   |                      |<-- done --------------------------|
 |                   |<-- IP, status -------|                  |                 |
 |<-- "Running at    |                      |                  |                 |
 |    34.x.x.x" -----|                      |                  |                 |
```

## File Storage Layout

```
~/.serverku/
├── config.yaml              # Global config (default provider, telegram API key, etc.)
├── keys/
│   ├── serverku_rsa          # SSH private key (auto-generated)
│   └── serverku_rsa.pub      # SSH public key
├── projects/
│   ├── my-project.yaml       # Project config
│   └── examapp.yaml          # Project config
└── state/
    ├── my-project.json        # Runtime state (VM ID, disk ID, IP, status)
    └── examapp.json           # Runtime state
```

**Project config** (YAML, user-editable):
```yaml
name: my-project
provider: gcp
region: asia-southeast1
zone: asia-southeast1-b
vm:
  size: e2-medium
  image: ubuntu-22-04
  spot: true
storage:
  enabled: true
  size_gb: 20
  mount_path: /data
compose_file: ./docker-compose.yml
startup_commands:
  - "docker compose -f /data/docker-compose.yml up -d"
notify:
  telegram_chat_id: ""
```

**Project state** (JSON, managed by serverku):
```json
{
  "project_name": "my-project",
  "status": "running",
  "vm_id": "1234567890",
  "disk_id": "serverku-my-project-data",
  "external_ip": "34.101.123.45",
  "provider": "gcp",
  "region": "asia-southeast1",
  "zone": "asia-southeast1-b",
  "created_at": "2024-01-15T10:30:00Z",
  "started_at": "2024-01-15T10:30:00Z"
}
```

## Network & Security

Each project VM gets:
- A dedicated firewall rule/security group
- Open ports: 22 (SSH), 80 (HTTP), 443 (HTTPS), 8080, 9000-9999
- SSH access only via serverku-generated key
- External IP assigned on creation, released on destruction

Future consideration: VPC/subnet per project for isolation, or configurable port list per project.
