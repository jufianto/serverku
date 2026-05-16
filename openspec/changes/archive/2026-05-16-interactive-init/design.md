## Context

Currently, the `serverku init <project>` command creates a static, boilerplate YAML file. Users must manually open this file in an editor, figure out the correct fields (e.g., whether to use `gcp` or `digitalocean`), and enter valid values for regions and sizes. This is error-prone and a poor developer experience.

## Goals / Non-Goals

**Goals:**
- Provide an interactive, wizard-like CLI experience for `serverku init`.
- Ask for Provider, Region, Machine Size, Storage settings, and Compose file path.
- Provide sensible defaults for all inputs.
- Still support non-interactive usage if a flag (e.g., `--non-interactive`) is passed.

**Non-Goals:**
- We will not dynamically fetch available regions/sizes from the cloud providers during initialization. This would require authenticated API clients during `init`, which is too slow and assumes credentials are ready. We will hardcode common sensible defaults.

## Decisions

**1. Prompting Library:**
We will use `github.com/charmbracelet/huh` for the interactive prompts.
- *Rationale*: `huh` provides a very modern, beautiful, and easy-to-use API for building forms in the terminal. It handles styling, validation, and terminal states cleanly.

**2. Form Structure:**
The form will ask sequentially:
1. Select Provider (Select list: `gcp`, `digitalocean`)
2. Select Region (Input string with default based on provider, e.g., `sgp1` for DO, `asia-southeast1` for GCP)
3. VM Size (Input string with default based on provider)
4. Enable Storage (Confirm prompt)
5. Storage Size (Input integer, only if storage enabled)
6. Compose File Path (Input string, default `./docker-compose.yml`)

## Risks / Trade-offs

- **[Risk] `huh` form breaking in non-interactive environments (CI/CD)** → *Mitigation*: We will detect if standard input is not a terminal, or provide a `--non-interactive` flag to bypass the form and use defaults or boilerplate.