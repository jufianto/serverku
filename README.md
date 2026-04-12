# serverku

On-demand cloud VM orchestrator. Define a project, keep the server off. When you need it, one command spins up a VM, attaches persistent storage, deploys your Docker stack. When done, tear it down. Pay only for storage when idle.

## Why

Cloud VMs cost money 24/7, even when nobody is using them. For projects that don't need constant uptime - staging environments, demo servers, side projects, client previews - you're burning money for nothing.

serverku solves this by treating VMs as disposable compute. Your project data lives on persistent block storage. When you need the server, serverku creates a VM, attaches your storage, deploys your Docker containers, and gives you an IP. When you're done, it tears everything down. You pay pennies for idle storage instead of dollars for a running VM.

## Features

- **On-demand VMs** - Spin up and tear down with a single command
- **Persistent block storage** - Project data survives VM destruction (optional, can run fully stateless)
- **Docker-native** - Bring your `docker-compose.yml`, serverku handles the rest
- **Multi-cloud** - GCP and DigitalOcean support (more providers planned)
- **SPOT/preemptible instances** - Use cheap spot instances for even more savings
- **CLI-first** - Clean command-line interface for all operations
- **Telegram notifications** - Optional notifications when servers go up/down
- **Open-source** - Self-hosted, customizable, no vendor lock-in

## How It Works

```
VM OFF (idle):
  [Block Storage: 20GB]  ~$0.80/mo
  [Project config: YAML]

serverku up my-project:
  1. Create VM (SPOT instance)
  2. Attach persistent disk
  3. Install Docker, mount disk
  4. docker compose up
  5. Return IP address
  Total time: ~5 minutes

serverku down my-project:
  1. docker compose down
  2. Detach disk
  3. Destroy VM
  Cost drops to storage only
```

## Quick Start

### Prerequisites

- Go 1.21+
- A cloud provider account (GCP or DigitalOcean)
- Cloud provider credentials configured locally

### Install

```bash
go install github.com/jufianto/serverku/cmd/serverku@latest
```

### Usage

```bash
# Initialize a new project
serverku init my-project

# Edit the generated config at ~/.serverku/projects/my-project.yaml

# Spin up the server
serverku up my-project

# Check status and get IP
serverku status my-project

# Tear down when done
serverku down my-project

# List all projects
serverku list

# Permanently delete project and its storage
serverku destroy my-project
```

### Project Configuration

```yaml
# ~/.serverku/projects/my-project.yaml
name: my-project
provider: gcp
region: asia-southeast1
zone: asia-southeast1-b

vm:
  size: e2-medium
  image: ubuntu-22-04
  spot: true

storage:
  enabled: true          # false = fully stateless VM
  size_gb: 20
  mount_path: /data

compose_file: ./docker-compose.yml

startup_commands:
  - "docker compose -f /data/docker-compose.yml up -d"

notify:
  telegram_chat_id: ""   # optional
```

## Supported Providers

| Provider | Status |
|----------|--------|
| Google Cloud Platform (GCP) | In progress |
| DigitalOcean | Planned |
| AWS | Planned |
| Hetzner | Planned |

## Architecture

```
serverku CLI
    |
    v
Orchestrator
    |
    +-- ProjectStore (YAML configs in ~/.serverku/)
    |
    +-- CloudProvider interface
    |       +-- GCP Provider (Compute Engine SDK)
    |       +-- DO Provider (godo SDK)
    |
    +-- Provisioner (SSH-based setup)
            +-- Docker installation
            +-- Disk mounting
            +-- Docker Compose deployment
```

## Cost Comparison

For a typical e2-medium (2 vCPU, 4GB) instance on GCP:

| Scenario | Always-on | serverku (4h/day usage) |
|----------|-----------|------------------------|
| VM cost | ~$25/mo | ~$3.30/mo (SPOT) |
| Storage | included | ~$0.80/mo (20GB PD) |
| **Total** | **~$25/mo** | **~$4.10/mo** |

## Development

```bash
# Clone
git clone https://github.com/jufianto/serverku.git
cd serverku

# Build
go build -o serverku ./cmd/serverku

# Run
./serverku --help
```

## License

MIT
