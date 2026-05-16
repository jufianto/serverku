## Why

Users often need to access internal services (like a database or admin panel) that should not be exposed to the public internet via the Caddy reverse proxy. Adding a `serverku tunnel` command allows secure local access to these remote internal services via SSH port forwarding.

## What Changes

- Add a new CLI command `serverku tunnel <project-name> <local-port>:<remote-port>`.
- The command will spawn an SSH process with the `-L` flag to forward the specified local port to the remote VM port securely.

## Capabilities

### New Capabilities
- `port-tunneling`: Provide secure local port forwarding to the remote VM via SSH.

### Modified Capabilities

## Impact

- **Code:** Adds `cmd/serverku/tunnel.go`.
- **Dependencies:** Relies on system `ssh` binary.
- **Systems:** Improves security by allowing users to access services without exposing them publicly.
