## Context

Sometimes users run internal services (databases, admin panels) on their remote VM that they don't want exposed to the internet. They need a secure way to access these ports from their local machine.

## Goals / Non-Goals

**Goals:**
- Provide a `serverku tunnel <project-name> <local-port>:<remote-port>` command.
- Set up a secure SSH tunnel forwarding the local port to the remote port.

**Non-Goals:**
- We will not implement persistent background tunneling daemons. The tunnel will stay open as long as the CLI command is running and blocking the terminal.

## Decisions

**1. SSH Port Forwarding:**
We will use `os/exec` to run `ssh -N -L <local-port>:localhost:<remote-port> -i <key> user@ip`.
- *Rationale*: Standard SSH port forwarding is secure, robust, and doesn't require any additional software on the remote VM. The `-N` flag prevents executing a remote command.

## Risks / Trade-offs

- **[Risk] Port already in use** → *Mitigation*: The `ssh` command will output an error if the local port is already bound, which will be visible to the user.
