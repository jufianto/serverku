## Why

When users deploy applications to remote VMs, they typically expect them to be accessible via a secure HTTPS domain. Currently, `serverku` only deploys raw containers exposing random ports. By integrating `serverku` with the `caddyku` reverse proxy CLI, we can automatically provide zero-touch, zero-downtime routing and automatic Let's Encrypt SSL certificates for deployed projects.

## What Changes

- Update `ProjectConfig` to support a `Router` block with multiple `Domains`.
- Modify the `SSHProvisioner` to download and install the `caddyku` binary on the remote VM.
- Instruct `caddyku` to bootstrap the global proxy network on the VM.
- Before starting the user's `docker-compose.yml`, run `caddyku init-app` to automatically wire the user's service into the proxy network and set up the Caddyfile rules for the configured domains.

## Capabilities

### New Capabilities
- `caddyku-integration`: Automated installation of a global reverse proxy and per-project automatic HTTPS domain routing via the `caddyku` CLI.

### Modified Capabilities
- `<none>`

## Impact

- **Code:** Modifies `internal/config/project.go` and significantly expands `internal/provisioner/provisioner.go`.
- **Dependencies:** Relies on downloading the external `caddyku` binary from GitHub Releases to the remote VM during provisioning.
- **Systems:** Transforms `serverku` from a simple Docker runner into a full-fledged PaaS with native HTTPS routing.
