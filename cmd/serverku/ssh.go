package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh <project-name>",
		Short: "Open an interactive SSH session to a project's VM",
		Long: `Retrieve the project's IP address and automatically open an SSH
connection using the serverku managed private key.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Load project state
			state, err := store.LoadState(name)
			if err != nil {
				return fmt.Errorf("failed to load state: %w", err)
			}

			if state.ExternalIP == "" || !state.IsRunning() {
				return fmt.Errorf("project %q is not running or has no external IP", name)
			}

			// Get the SSH private key
			privKeyPath, err := projectSSHKeyPath(name)
			if err != nil {
				return fmt.Errorf("failed to get SSH private key: %w", err)
			}

			// Verify ssh binary exists
			sshBin, err := exec.LookPath("ssh")
			if err != nil {
				return fmt.Errorf("ssh client not found in PATH. Please install an SSH client")
			}

			fmt.Printf("Connecting to %s@%s...\n", "serverku", state.ExternalIP)

			// Prepare the SSH command
			sshCmd := exec.Command(sshBin,
				"-i", privKeyPath,
				"-o", "IdentitiesOnly=yes",
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "LogLevel=ERROR", // Suppress some warnings
				fmt.Sprintf("serverku@%s", state.ExternalIP),
			)

			// Bind standard I/O for interactive session
			sshCmd.Stdin = os.Stdin
			sshCmd.Stdout = os.Stdout
			sshCmd.Stderr = os.Stderr

			// Run it synchronously
			if err := sshCmd.Run(); err != nil {
				// Don't wrap the error since ssh exit codes usually mean the remote command failed or connection died,
				// which is normal operation for an interactive shell wrapper.
				return fmt.Errorf("ssh session ended: %w", err)
			}

			return nil
		},
	}
}
