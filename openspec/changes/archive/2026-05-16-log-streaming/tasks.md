## 1. CLI Command Setup

- [x] 1.1 Create `cmd/serverku/logs.go`.
- [x] 1.2 Implement `newLogsCmd()` for `serverku logs <project-name>`.
- [x] 1.3 Register `newLogsCmd()` in `cmd/serverku/main.go`.

## 2. Log Streaming Implementation

- [x] 2.1 Load project config and state, then validate the project has a running VM and external IP.
- [x] 2.2 Resolve the remote compose directory based on storage configuration.
- [x] 2.3 Resolve the managed SSH private key path and validate the system `ssh` binary exists.
- [x] 2.4 Execute `ssh` with `cd <compose-dir> && docker compose logs -f`, binding local stdin/stdout/stderr.
