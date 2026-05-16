## Why

Currently, if a user wants to access their running VM, they have to manually find the IP address from the output of `serverku status <project>` or `serverku up <project>`, and then manually type the SSH command pointing to the private key managed by `serverku`. Adding a `serverku ssh <project-name>` command will greatly improve the developer experience by automatically resolving the IP, locating the SSH key, and opening an interactive shell session to the remote server.

## What Changes

- Add a new CLI command `serverku ssh <project-name>`.
- The command will load the project's state to retrieve the current external IP address.
- The command will use the generated `serverku_rsa` private key located in the config directory.
- It will spawn an interactive sub-process using the system's `ssh` binary, attaching `os.Stdin`, `os.Stdout`, and `os.Stderr`.

## Capabilities

### New Capabilities
- `ssh-command`: Support for opening an interactive SSH session to a running project's VM directly from the CLI.

### Modified Capabilities
- `<none>`

## Impact

- **Code:** Adds a new file `cmd/serverku/ssh.go` and registers the `ssh` command in `main.go`. Uses `os/exec` to invoke the system's `ssh` client.
- **Dependencies:** Relies on the host system having an `ssh` client installed in the `$PATH`.
- **Systems:** Improves user interaction with the managed VMs.
