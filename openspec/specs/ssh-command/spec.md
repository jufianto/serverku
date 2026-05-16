## ADDED Requirements

### Requirement: Interactive SSH Command
The CLI SHALL provide a `serverku ssh <project-name>` command that opens an interactive shell session to the project's VM.

#### Scenario: Successful connection
- **WHEN** the user runs `serverku ssh <project>` for a running project
- **THEN** the system SHALL spawn an `ssh` process connecting to the VM's external IP as user `serverku` using the `serverku_rsa` private key, and attach Stdin/Stdout/Stderr.

#### Scenario: Project not running
- **WHEN** the user runs `serverku ssh <project>` but the project state shows no external IP (e.g., stopped or destroyed)
- **THEN** the command SHALL return an error indicating the project is not running.

#### Scenario: Missing SSH client
- **WHEN** the user runs `serverku ssh <project>` but the `ssh` binary is not found in the system `$PATH`
- **THEN** the command SHALL return a user-friendly error instructing them to install an SSH client.
