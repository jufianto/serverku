## 1. CLI Command Setup

- [x] 1.1 Create `cmd/serverku/tunnel.go`.
- [x] 1.2 Implement `newTunnelCmd()` for `serverku tunnel <project-name> <local-port>:<remote-port>`.
- [x] 1.3 Register `newTunnelCmd()` in `cmd/serverku/main.go`.

## 2. Tunnel Implementation

- [x] 2.1 Validate and parse the `<local-port>:<remote-port>` mapping.
- [x] 2.2 Load project state and validate the project has a running VM and external IP.
- [x] 2.3 Resolve the managed SSH private key path and validate the system `ssh` binary exists.
- [x] 2.4 Execute `ssh -N -L <local-port>:localhost:<remote-port>` and bind local stdin/stdout/stderr.
