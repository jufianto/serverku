package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <project-name>",
		Short: "Stream Docker Compose logs from a project's VM",
		Long: `Retrieve the project's IP address and stream docker compose logs
from the serverku deployment directory over SSH.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := store.LoadProject(name)
			if err != nil {
				return fmt.Errorf("failed to load project: %w", err)
			}

			state, err := store.LoadState(name)
			if err != nil {
				return fmt.Errorf("failed to load state: %w", err)
			}

			if state.ExternalIP == "" || !state.IsRunning() {
				return fmt.Errorf("project %q is not running or has no external IP", name)
			}

			composeDir := "/home/serverku"
			if cfg.Storage.Enabled && cfg.Storage.MountPath != "" {
				composeDir = cfg.Storage.MountPath
			}

			privKeyPath, err := projectSSHKeyPath(name)
			if err != nil {
				return fmt.Errorf("failed to get SSH private key: %w", err)
			}

			sshBin, err := exec.LookPath("ssh")
			if err != nil {
				return fmt.Errorf("ssh client not found in PATH. Please install an SSH client")
			}

			fmt.Printf("Streaming logs for %q from %s...\n", name, state.ExternalIP)

			remoteCmd := fmt.Sprintf("cd %q && docker compose logs -f", composeDir)
			logsCmd := exec.CommandContext(cmd.Context(), sshBin,
				"-i", privKeyPath,
				"-o", "IdentitiesOnly=yes",
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "LogLevel=ERROR",
				fmt.Sprintf("serverku@%s", state.ExternalIP),
				remoteCmd,
			)

			logsCmd.Stdin = os.Stdin
			logsCmd.Stdout = os.Stdout
			logsCmd.Stderr = os.Stderr

			if err := logsCmd.Run(); err != nil {
				return fmt.Errorf("log streaming ended: %w", err)
			}

			return nil
		},
	}
}
