## Why

Users currently have to SSH into their servers and run docker commands manually to view application logs, which slows down debugging. Adding a native `serverku logs` command will make it much faster and easier to inspect container output directly from the local terminal.

## What Changes

- Add a new CLI command `serverku logs <project-name>`.
- The command will establish an SSH connection to the running VM and execute `docker compose logs -f` in the project directory, streaming the output to the local stdout.

## Capabilities

### New Capabilities
- `log-streaming`: Stream Docker container logs directly from the remote VM to the local CLI.

### Modified Capabilities

## Impact

- **Code:** Adds a new `cmd/serverku/logs.go` command and registers it in `main.go`.
- **Dependencies:** Relies on system `ssh` binary.
- **Systems:** Improves developer experience and observability.
