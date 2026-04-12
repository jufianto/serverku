# Implementation Plan

## Phase 1: Project Restructure

**Goal:** Move from flat `main.go` + `pkg/` layout to proper Go project structure.

### Tasks

1. Create directory structure:
   ```
   cmd/serverku/main.go
   internal/provider/
   internal/config/
   internal/orchestrator/
   internal/provisioner/
   internal/notify/
   ```

2. Add cobra dependency and set up root command with subcommands (stubs)

3. Move existing Telegram code to `internal/notify/telegram.go` (adapt, don't just copy)

4. Move existing Firestore code to `internal/legacy/` (keep as reference, not used in new architecture)

5. Keep `terraform/` directory as-is for reference

6. Update `go.mod` with new dependencies:
   - `github.com/spf13/cobra`
   - `google.golang.org/api/compute/v1`
   - `golang.org/x/crypto/ssh`

### Deliverables
- `cmd/serverku/main.go` with cobra root command
- Stub subcommands: init, up, down, status, list, destroy
- Existing code preserved in `internal/legacy/`
- Project builds and `serverku --help` works

---

## Phase 2: Core Interfaces & Models

**Goal:** Define the contracts that all components will implement.

### Tasks

1. **CloudProvider interface** (`internal/provider/provider.go`):
   - `CreateVM`, `DestroyVM`, `StartVM`, `StopVM`
   - `GetVMStatus`, `GetExternalIP`, `WaitForReady`
   - `CreateDisk`, `DeleteDisk`, `AttachDisk`, `DetachDisk`

2. **Data models** (`internal/provider/models.go`):
   - `VMConfig` - input for creating a VM
   - `VM` - result of creating a VM (ID, name, zone)
   - `VMStatus` - current state (running, stopped, terminated)
   - `DiskConfig` - input for creating a disk
   - `Disk` - result of creating a disk (ID, name, size)

3. **ProjectConfig model** (`internal/config/project.go`):
   - YAML struct matching the project config format
   - Validation method
   - Default values

4. **ProjectState model** (`internal/config/state.go`):
   - JSON struct for runtime state
   - Status enum: `stopped`, `starting`, `running`, `stopping`, `error`

5. **ProjectStore interface** (`internal/config/store.go`):
   - `Save(config)`, `Load(name)`, `Delete(name)`, `List()`
   - `SaveState(state)`, `LoadState(name)`, `DeleteState(name)`
   - Implementation: filesystem-based (YAML/JSON files in `~/.serverku/`)

### Deliverables
- All interfaces and models defined with proper Go doc comments
- No implementation yet, just contracts
- Unit tests for config validation and serialization

---

## Phase 3: Project Config & Store

**Goal:** Implement project configuration management.

### Tasks

1. Implement `FileStore` (filesystem-based ProjectStore):
   - Create `~/.serverku/` directory structure on first use
   - Read/write YAML project configs
   - Read/write JSON state files
   - List all projects by scanning config directory

2. Implement `serverku init` command:
   - Interactive prompts: project name, provider, region, VM size, storage yes/no
   - Generate YAML config file
   - Validate no name collision

3. Implement `serverku list` command:
   - Read all project configs
   - Read corresponding state files
   - Display table: name, provider, status, IP, last started

4. SSH key generation:
   - Generate RSA key pair on first run if not exists
   - Store in `~/.serverku/keys/`

### Deliverables
- Working `serverku init my-project` creates a config file
- Working `serverku list` shows all projects
- SSH keys auto-generated
- Unit tests for store operations

---

## Phase 4: GCP Provider

**Goal:** Implement the CloudProvider interface for Google Cloud Platform.

### Tasks

1. **Authentication:**
   - Use Application Default Credentials (ADC)
   - User runs `gcloud auth application-default login` before using serverku
   - Read project ID from project config (not global)

2. **CreateVM:**
   - Use `google.golang.org/api/compute/v1`
   - Create instance with:
     - Machine type from config
     - Ubuntu 22.04 image
     - SPOT scheduling if `spot: true`
     - SSH key in instance metadata
     - Tags for firewall
   - Return VM ID

3. **DestroyVM:**
   - Delete the compute instance
   - Wait for operation to complete

4. **CreateDisk:**
   - Create a persistent disk in the same zone
   - Name format: `serverku-{project-name}-data`
   - Size from config
   - Type: `pd-standard` (cheapest)

5. **AttachDisk / DetachDisk:**
   - Attach persistent disk to running instance
   - Detach before VM deletion

6. **DeleteDisk:**
   - Permanently delete persistent disk (for `serverku destroy`)

7. **GetVMStatus / GetExternalIP / WaitForReady:**
   - Poll instance status
   - Extract external IP from network interface
   - WaitForReady: poll until status is RUNNING (with timeout)

8. **Network setup:**
   - Create a firewall rule for the project if not exists
   - Name format: `serverku-{project-name}-fw`
   - Allow: TCP 22, 80, 443, 8080, 9000-9999
   - Use target tags for scoping

### Deliverables
- Full GCP provider implementation
- Integration tests (tagged `//go:build integration`)
- Manual test: create and destroy a VM via the provider directly

---

## Phase 5: Orchestrator

**Goal:** Implement the core up/down/status logic that coordinates provider and provisioner.

### Tasks

1. **Orchestrator struct:**
   ```go
   type Orchestrator struct {
       store      config.ProjectStore
       providers  map[string]provider.CloudProvider
       provisioner provisioner.Provisioner
       notifier   notify.Notifier  // optional
   }
   ```

2. **Up(ctx, projectName) method:**
   - Load config and state
   - Validate project is not already running
   - Create disk if storage enabled and disk doesn't exist
   - Create VM
   - Attach disk if storage enabled
   - Wait for VM ready
   - Get external IP
   - Run provisioner
   - Save state
   - Send notification

3. **Down(ctx, projectName) method:**
   - Load config and state
   - Validate project is running
   - Run provisioner teardown (docker compose down)
   - Detach disk if storage enabled
   - Destroy VM
   - Update state
   - Send notification

4. **Status(ctx, projectName) method:**
   - Load state
   - If VM ID exists, verify with provider (VM might have been terminated externally)
   - Return current status

5. **Destroy(ctx, projectName) method:**
   - Down() if running
   - Delete disk if exists
   - Delete state file
   - Delete config file (with confirmation)

6. **Error handling and recovery:**
   - If VM creation fails after disk creation, don't delete disk
   - If provisioning fails, destroy VM but keep disk
   - Save state at each step so recovery is possible
   - Log each step for debugging

### Deliverables
- Orchestrator with up/down/status/destroy
- Unit tests with mocked provider and provisioner
- Error recovery paths tested

---

## Phase 6: CLI Commands

**Goal:** Wire cobra commands to the orchestrator.

### Tasks

1. **Root command** (`cmd/serverku/main.go`):
   - Version flag
   - Global flags: `--config-dir` (default `~/.serverku/`), `--verbose`
   - Initialize store, provider, orchestrator in PersistentPreRun

2. **`init` command:**
   - Flags: `--provider`, `--region`, `--size`, `--storage`, `--no-storage`
   - Interactive mode if flags not provided (use promptui or survey)
   - Create config YAML

3. **`up` command:**
   - Arg: project name (required)
   - Show progress: "Creating VM...", "Attaching storage...", "Provisioning..."
   - Output: IP address and status on completion

4. **`down` command:**
   - Arg: project name (required)
   - Confirmation prompt (skip with `--force`)
   - Show progress

5. **`status` command:**
   - Arg: project name (required)
   - Output: status, IP, uptime, provider, region, VM size

6. **`list` command:**
   - No args
   - Table output: name | provider | status | IP | storage

7. **`destroy` command:**
   - Arg: project name (required)
   - Double confirmation ("This will permanently delete all data. Type project name to confirm:")
   - `--force` to skip confirmation

### Deliverables
- All CLI commands wired to orchestrator
- Clean output with progress indicators
- `--help` for every command
- Manual end-to-end test: init -> up -> status -> down -> destroy

---

## Phase 7: Provisioner

**Goal:** SSH into newly created VMs and set up Docker + deploy containers.

### Tasks

1. **SSH client wrapper:**
   - Connect using serverku SSH key
   - Default user: based on provider (GCP = username from gcloud, DO = root)
   - Retry connection with exponential backoff (VM might not be ready for SSH immediately)
   - Timeout after 5 minutes

2. **Docker installation script:**
   ```bash
   curl -fsSL https://get.docker.com | sh
   sudo usermod -aG docker $USER
   sudo systemctl enable docker
   ```

3. **Disk mounting (if storage enabled):**
   ```bash
   # Check if disk needs formatting
   sudo blkid /dev/sdb || sudo mkfs.ext4 /dev/sdb
   sudo mkdir -p /data
   sudo mount /dev/sdb /data
   # Add to fstab
   echo '/dev/sdb /data ext4 defaults 0 2' | sudo tee -a /etc/fstab
   ```

4. **Docker Compose deployment:**
   - Transfer docker-compose.yml to VM via SCP
   - Place in mount_path (e.g., `/data/docker-compose.yml`)
   - Run `docker compose up -d`
   - Verify containers started

5. **Teardown method:**
   - `docker compose down`
   - Unmount disk: `sudo umount /data`
   - Clean shutdown

### Deliverables
- Working provisioner that sets up a VM from scratch
- Docker + compose running within ~3 minutes of VM creation
- Teardown cleanly stops everything before VM destruction

---

## Phase 8: Telegram Notifications

**Goal:** Optional notifications when project VMs go up/down.

### Tasks

1. **Notifier interface** (`internal/notify/notify.go`):
   ```go
   type Notifier interface {
       SendUp(project string, ip string) error
       SendDown(project string) error
       SendError(project string, err error) error
   }
   ```

2. **Telegram implementation** (`internal/notify/telegram.go`):
   - Reuse existing telegram-bot-api code
   - Read API key from global config (`~/.serverku/config.yaml`)
   - Chat ID from project config
   - Message templates:
     - Up: "serverku: Project {name} is running at {ip}"
     - Down: "serverku: Project {name} has been shut down"
     - Error: "serverku: Error with project {name}: {error}"

3. **NoopNotifier** for when notifications are not configured

4. **Global config** (`~/.serverku/config.yaml`):
   ```yaml
   telegram:
     api_key: "bot123:ABC..."
   ```

### Deliverables
- Telegram notifications on up/down/error
- Graceful no-op when not configured
- Reuses existing telegram code from legacy

---

## Phase 9: Documentation & Polish

**Goal:** Make the project ready for open-source release.

### Tasks

1. Update README.md with final accurate commands and output examples
2. Add CONTRIBUTING.md
3. Add LICENSE file (MIT)
4. Add example project configs in `configs/`
5. Add `.goreleaser.yml` for release builds
6. Add GitHub Actions CI (build + test)
7. Clean up go.mod - remove unused dependencies

### Deliverables
- Professional README with badges
- CI passing
- Release binary available
- Example configs for common setups (Next.js + Postgres, Laravel + MySQL, static site)

---

## Phase 10: DigitalOcean Provider

**Goal:** Add DigitalOcean as a second cloud provider.

### Tasks

1. Implement CloudProvider interface using `github.com/digitalocean/godo`:
   - CreateVM -> Create Droplet
   - DestroyVM -> Delete Droplet
   - CreateDisk -> Create Volume
   - AttachDisk -> Attach Volume to Droplet
   - DetachDisk -> Detach Volume
   - DeleteDisk -> Delete Volume
   - SSH key management via DO API

2. Authentication:
   - DO API token from global config or environment variable

3. Provider-specific considerations:
   - DO doesn't have SPOT instances - use regular droplets
   - DO volumes are region-scoped (not zone-scoped like GCP)
   - DO has built-in firewall API (Cloud Firewalls)

4. Update `serverku init` to support `--provider digitalocean`

### Deliverables
- Working DO provider
- Integration tests
- Documentation updated with DO examples

---

## Phase 11: Billing Notifications & Cost Monitoring

**Goal:** Fetch billing/cost data from cloud providers and notify users via Telegram.

This was part of the original project vision (see `.planning/usecase.png`). The idea is to alert users when they have VMs running that are costing money, and optionally send scheduled billing summaries.

### Tasks

1. **Billing interface** (`internal/billing/billing.go`):
   ```go
   type BillingProvider interface {
       GetCurrentCost(ctx context.Context, projectName string) (*CostInfo, error)
       GetRunningResources(ctx context.Context) ([]RunningResource, error)
   }

   type CostInfo struct {
       VMCostPerHour    float64
       DiskCostPerMonth float64
       EstimatedMonthly float64
       Currency         string
   }

   type RunningResource struct {
       ProjectName string
       ResourceType string  // "vm", "disk"
       Provider    string
       CostPerHour float64
       RunningFor  time.Duration
   }
   ```

2. **GCP billing** (`internal/billing/gcp.go`):
   - Use GCP Cloud Billing API or Compute Engine pricing
   - Calculate cost based on machine type, region, spot vs on-demand
   - Track uptime from project state `started_at` timestamp

3. **DigitalOcean billing** (`internal/billing/digitalocean.go`):
   - Use DO Balance API (`/v2/customers/my/balance`)
   - Calculate per-droplet cost from droplet size

4. **Notification triggers:**
   - **On `serverku status`**: Show estimated cost alongside status info
   - **Active VM alert**: When a VM has been running for X hours (configurable), send Telegram notification
     ```
     "serverku: Project 'examapp' has been running for 12 hours. Estimated cost so far: $0.16. Run 'serverku down examapp' to stop."
     ```
   - **Scheduled summary**: Optional cron-like check (if serverku is running as a daemon/service) that sends periodic billing summaries
     ```
     "serverku daily summary: 2 VMs running. Today's cost: $0.45. Monthly estimate: $13.50."
     ```

5. **Project config for billing alerts:**
   ```yaml
   notify:
     telegram_chat_id: "131109047"
     billing_alert_hours: 8     # Alert if VM running longer than 8 hours
     daily_summary: true        # Send daily cost summary
   ```

6. **CLI command:**
   ```bash
   serverku cost                    # Show cost summary for all running projects
   serverku cost my-project         # Show cost for specific project
   ```

### Deliverables
- Cost estimation displayed in `serverku status` output
- `serverku cost` command
- Telegram billing alerts when VM exceeds configured runtime
- Optional daily summary (requires background process or cron)

---

## Phase 12: SPOT Termination Handling

**Goal:** Gracefully handle when cloud providers reclaim SPOT/preemptible VMs.

From the original planning notes: "the data must not be deleted when VM restart" - this is inherently handled by the persistent disk architecture (disk survives VM deletion). But we need to handle the state tracking side.

### Tasks

1. **State reconciliation:**
   - When `serverku status` is called, verify with the cloud provider that the VM still exists
   - If VM was terminated (SPOT reclaimed), update local state to `stopped`
   - Disk remains intact - no data loss

2. **GCP preemption detection:**
   - Check instance status: if `TERMINATED` and scheduling was SPOT, VM was reclaimed
   - Log the event

3. **Auto-recovery (optional, future):**
   - If configured, automatically `serverku up` when SPOT termination is detected
   - Requires a watcher/daemon (out of scope for v0.1.0)
   - Config:
     ```yaml
     vm:
       spot: true
       auto_recover: true   # Re-create VM if SPOT terminated
     ```

4. **Notification on SPOT termination:**
   - If Telegram is configured, send alert:
     ```
     "serverku: Project 'examapp' VM was reclaimed (SPOT termination). Data is safe on persistent disk. Run 'serverku up examapp' to restart."
     ```

### Deliverables
- State reconciliation in `serverku status`
- Telegram notification on SPOT termination
- Documentation on SPOT behavior and expectations
- Auto-recovery designed but deferred to future version
