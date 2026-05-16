## Context

Users want a fast way to view the stdout/stderr of their containers running on the remote VM. Currently, they must SSH in and run `docker compose logs -f` manually.

## Goals / Non-Goals

**Goals:**
- Provide a `serverku logs <project-name>` command.
- Automatically connect via SSH and execute `docker compose logs -f` in the project's deployment directory.
- Stream the output back to the user's terminal in real-time.

**Non-Goals:**
- We are not building a centralized log aggregator (like ELK or Loki). This is strictly for real-time tailing of the active docker compose stack.

## Decisions

**1. SSH Execution:**
We will use `os/exec` to run `ssh -i <key> user@ip "cd <dir> && docker compose logs -f"`.
- *Rationale*: This naturally pipes the remote stdout/stderr directly into our local stdout/stderr, providing instant streaming without needing to parse the streams in Go.

## Risks / Trade-offs

- **[Risk] SSH connection drops** → *Mitigation*: The `exec.Cmd.Run()` will return an error which we can pass back to the user gracefully.
