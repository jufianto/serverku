## Why

Currently, `serverku init <project>` generates a static, boilerplate YAML file that users must manually open and edit to configure their provider, region, VM size, and storage settings. This creates friction and a poor onboarding experience, especially for new users who may not know the valid options. Making `init` interactive provides a frictionless, guided setup experience.

## What Changes

- Modify `cmd/serverku/init.go` to use an interactive prompting library (like `huh` or `survey`).
- Prompt the user for required configuration options: Cloud Provider, Region, VM Size, Storage (Enabled/Size).
- Automatically construct the `ProjectConfig` based on the user's answers and write the finished YAML to disk.
- If the user uses a `--non-interactive` flag or pipes input, fallback to the current boilerplate generation.

## Capabilities

### New Capabilities
- `interactive-init`: Provide a CLI wizard to guide users through creating a new project configuration.

### Modified Capabilities
- `<none>`

## Impact

- **Code:** Major changes to `cmd/serverku/init.go`.
- **Dependencies:** Adds a new dependency for interactive CLI prompts (e.g., `github.com/charmbracelet/huh` or `github.com/AlecAivazis/survey/v2`).
- **Systems:** Improves user experience; the generated YAML remains fully compatible with the rest of the system.
