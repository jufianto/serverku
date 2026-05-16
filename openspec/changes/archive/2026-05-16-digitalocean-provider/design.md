## Context

`serverku` relies on the `internal/provider.CloudProvider` interface to abstract interactions with different cloud platforms. The interface expects methods to create, get, stop, delete VMs, and to handle block storage (create, attach, detach, delete).

The orchestrator (`internal/orchestrator/orchestrator.go`) uses this interface during the `Up()`, `Down()`, and `Destroy()` lifecycles. Currently, only the GCP implementation exists (`internal/provider/gcp`).

To add DigitalOcean support, we need to:
1. Create a new package `internal/provider/digitalocean`.
2. Implement the `CloudProvider` interface using the `github.com/digitalocean/godo` client.
3. Wire this into the `newProviderFactory` in `cmd/serverku/lifecycle.go`.

## Goals / Non-Goals

**Goals:**
- Implement all methods of the `internal/provider.CloudProvider` interface for DigitalOcean.
- Support provisioning and tearing down Droplets.
- Support provisioning, attaching, and detaching Block Storage volumes.
- Use `DIGITALOCEAN_TOKEN` (or similar environment variable/auth mechanism) to authenticate the client.

**Non-Goals:**
- Support for DigitalOcean managed databases, Load Balancers, or Kubernetes.
- We are not changing the core provisioning logic (SSH/Docker setup), only the infrastructure layer.

## Decisions

**1. DigitalOcean Client Library:**
We will use the official `github.com/digitalocean/godo` package.
- *Rationale*: It is the standard, well-maintained Go client for the DO API.

**2. Authentication:**
We will use an environment variable `DIGITALOCEAN_TOKEN` to instantiate the DO client, similar to how the GCP provider relies on standard GCP auth mechanisms.
- *Rationale*: This is the standard 12-factor app way to handle secrets and avoids needing to put secrets in the `serverku` config files explicitly for now.

**3. Resource Naming and Tagging:**
Droplets and Volumes will be named using the `serverku` project name (e.g., `serverku-<project-name>`). We will also apply tags (e.g., `serverku-managed`) to make it easier to identify and clean up resources created by the tool.

**4. Region and Size Mapping:**
The `serverku` config specifies `Region` (e.g., `nyc1`) and `VM.Size` (e.g., `s-1vcpu-1gb`). These map cleanly to DigitalOcean's API concepts without translation needed. Spot instances are not supported natively by DO in the same way GCP preemptible VMs are, so `VM.Spot` will be ignored or return an error if `true`.

## Risks / Trade-offs

- **[Risk] DigitalOcean API Rate Limits** → *Mitigation*: Our orchestrator operations are typically infrequent (user-driven `up` or `down`), so rate limits shouldn't be an issue in normal use.
- **[Risk] Block Storage Attachment delays** → *Mitigation*: The DO API sometimes takes a few seconds to fully attach a volume. We may need to poll the Droplet/Volume status to ensure it's attached before the `Provisioner` attempts to mount it.
