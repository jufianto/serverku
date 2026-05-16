## ADDED Requirements

### Requirement: Log Streaming Command
The CLI SHALL provide a `serverku logs <project-name>` command that streams the logs of the remote docker-compose stack.

#### Scenario: Project is running
- **WHEN** the user runs `serverku logs <project>` for a running project
- **THEN** the CLI SHALL connect via SSH and stream the output of `docker compose logs -f` to the local terminal.

#### Scenario: Project is not running
- **WHEN** the user runs `serverku logs <project>` but the project has no external IP
- **THEN** the CLI SHALL return an error indicating the project is not running.
