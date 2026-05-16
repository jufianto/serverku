## Why

`serverku` currently only has a working provider implementation for Google Cloud Platform (GCP). The project configuration validation already accepts `digitalocean` as a provider, but executing the lifecycle returns a "not yet implemented" stub. Adding a full DigitalOcean provider implementation will give users more flexibility and lower-cost options for hosting their applications via `serverku`.

## What Changes

- Implement the `internal/provider.Provider` interface for DigitalOcean (Droplet creation, deletion, status checking).
- Plumb the DigitalOcean implementation into `cmd/serverku/lifecycle.go` where it is currently stubbed.
- Support provisioning droplets with attached block storage if storage is enabled in the project config.
- Utilize the official `digitalocean/godo` Go client library.

## Capabilities

### New Capabilities
- `digitalocean-provider`: Support for creating, managing, and destroying DigitalOcean Droplets and Block Storage volumes via the `serverku` orchestrator.

### Modified Capabilities

- `<none>`: The core provider interface and SSH provisioner requirements are not changing; this is adding a new implementation.

## Impact

- **Code:** Adds a new package `internal/provider/digitalocean`. Modifies `cmd/serverku/lifecycle.go` to instantiate the new provider.
- **Dependencies:** Will introduce the `github.com/digitalocean/godo` dependency to the project.
- **Systems:** Users will be able to deploy their workloads to DigitalOcean using existing project configuration structures.
