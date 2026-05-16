## ADDED Requirements

### Requirement: Interactive Init Command
The `serverku init <project>` command SHALL provide an interactive form to guide users through configuring their project when run in a TTY environment.

#### Scenario: User runs init interactively
- **WHEN** the user runs `serverku init <project>` in a terminal
- **THEN** the CLI SHALL prompt for Provider, Region, VM Size, Storage options, and Compose file path, using sensible defaults for each.

#### Scenario: Generated configuration
- **WHEN** the user completes the interactive form
- **THEN** the CLI SHALL generate the `~/.serverku/projects/<project>.yaml` file populated with the user's choices.

#### Scenario: Non-interactive fallback
- **WHEN** the user runs `serverku init <project> --non-interactive` OR the environment is not a TTY
- **THEN** the CLI SHALL bypass the interactive prompts and generate the default boilerplate YAML file.
