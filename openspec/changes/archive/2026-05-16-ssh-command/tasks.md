## 1. CLI Command Setup

- [x] 1.1 Create `cmd/serverku/ssh.go` file.
- [x] 1.2 Implement `newSSHCmd()` that returns a `*cobra.Command` for `serverku ssh <project-name>`.
- [x] 1.3 Add `newSSHCmd()` to the root command in `cmd/serverku/main.go`.

## 2. Implementation

- [x] 2.1 Inside `newSSHCmd`, load the project state to get the `ExternalIP`. If empty or not running, return an error.
- [x] 2.2 Get the SSH private key path using `store.GetSSHPrivateKeyPath()`.
- [x] 2.3 Verify `exec.LookPath("ssh")` succeeds, returning a helpful error if the `ssh` binary is missing.
- [x] 2.4 Create the `exec.Command` with args: `-i <private-key>`, `-o StrictHostKeyChecking=accept-new`, `serverku@<ip>`.
- [x] 2.5 Bind `cmd.Stdin = os.Stdin`, `cmd.Stdout = os.Stdout`, `cmd.Stderr = os.Stderr`.
- [x] 2.6 Run the command using `cmd.Run()`.