package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newTunnelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tunnel <project-name> <local-port>:<remote-port>",
		Short: "Forward a local port to a project's VM over SSH",
		Long: `Open an SSH tunnel to a running project VM. The command blocks while
the tunnel is active and exits when interrupted.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			localPort, remotePort, err := parsePortMapping(args[1])
			if err != nil {
				return err
			}

			state, err := store.LoadState(name)
			if err != nil {
				return fmt.Errorf("failed to load state: %w", err)
			}

			if state.ExternalIP == "" || !state.IsRunning() {
				return fmt.Errorf("project %q is not running or has no external IP", name)
			}

			privKeyPath, err := store.GetSSHPrivateKeyPath()
			if err != nil {
				return fmt.Errorf("failed to get SSH private key: %w", err)
			}

			sshBin, err := exec.LookPath("ssh")
			if err != nil {
				return fmt.Errorf("ssh client not found in PATH. Please install an SSH client")
			}

			forward := fmt.Sprintf("%d:localhost:%d", localPort, remotePort)
			fmt.Printf("Forwarding localhost:%d to %s:%d for project %q...\n", localPort, state.ExternalIP, remotePort, name)
			fmt.Println("Press Ctrl+C to close the tunnel.")

			tunnelCmd := exec.CommandContext(cmd.Context(), sshBin,
				"-N",
				"-L", forward,
				"-i", privKeyPath,
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "LogLevel=ERROR",
				fmt.Sprintf("serverku@%s", state.ExternalIP),
			)

			tunnelCmd.Stdin = os.Stdin
			tunnelCmd.Stdout = os.Stdout
			tunnelCmd.Stderr = os.Stderr

			if err := tunnelCmd.Run(); err != nil {
				return fmt.Errorf("tunnel ended: %w", err)
			}

			return nil
		},
	}
}

func parsePortMapping(mapping string) (int, int, error) {
	parts := strings.Split(mapping, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid port mapping %q, expected <local-port>:<remote-port>", mapping)
	}

	localPort, err := parsePort(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid local port: %w", err)
	}

	remotePort, err := parsePort(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid remote port: %w", err)
	}

	return localPort, remotePort, nil
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}
	return port, nil
}
