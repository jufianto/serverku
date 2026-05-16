## ADDED Requirements

### Requirement: Port Tunneling Command
The CLI SHALL provide a `serverku tunnel <project-name> <local-port>:<remote-port>` command to securely forward ports.

#### Scenario: Successful tunnel
- **WHEN** the user runs the tunnel command for a valid project
- **THEN** the CLI SHALL establish an SSH tunnel forwarding traffic from the local port to the remote port and block until interrupted by the user.

#### Scenario: Project not running
- **WHEN** the user runs the tunnel command but the project has no external IP
- **THEN** the CLI SHALL return an error indicating the project is not running.
