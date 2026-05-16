## 1. Setup

- [x] 1.1 Add `github.com/charmbracelet/huh` dependency to the project (`go get`).
- [x] 1.2 Add `--non-interactive` flag (boolean) to the `newInitCmd()` in `cmd/serverku/init.go`.

## 2. Interactive Form Implementation

- [x] 2.1 Create the `runInteractiveInit` function in `cmd/serverku/init.go`.
- [x] 2.2 Define the `huh.Form` with groups for Provider, Region, VM Size, Storage, and Compose File.
- [x] 2.3 Implement conditional logic in the form so that default values for Region and VM Size change based on the selected Provider.
- [x] 2.4 Execute the form and map the collected answers to a new `config.ProjectConfig` struct.

## 3. Integration

- [x] 3.1 Update the `RunE` logic in `newInitCmd()` to check if os.Stdin is a terminal and the `--non-interactive` flag is false.
- [x] 3.2 If interactive, call `runInteractiveInit()` to generate the config. If not, fallback to the existing boilerplate logic.
- [x] 3.3 Ensure the final generated config is saved using `store.SaveProject()`.