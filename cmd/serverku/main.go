package main

import (
	"fmt"
	"os"

	"github.com/jufianto/serverku/internal/config"
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	configDir string
	verbose   bool
	store     *config.Store
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "serverku",
		Short: "On-demand cloud VM orchestrator",
		Long: `serverku creates and destroys cloud VMs on demand.
Define a project, keep the server off. When you need it, one command spins up
a VM, attaches persistent storage, and deploys your Docker stack.
When done, tear it down. Pay only for storage when idle.`,
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip store initialization for help and version commands
			if cmd.Name() == "help" || cmd.Name() == "version" {
				return nil
			}

			dir := configDir
			if dir == "" {
				d, err := config.DefaultBaseDir()
				if err != nil {
					return err
				}
				dir = d
			}

			s, err := config.NewStore(dir)
			if err != nil {
				return fmt.Errorf("failed to initialize config store: %w", err)
			}
			store = s
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "", "config directory (default: ~/.serverku/)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Register subcommands
	rootCmd.AddCommand(
		newInitCmd(),
		newUpCmd(),
		newDownCmd(),
		newStatusCmd(),
		newListCmd(),
		newDestroyCmd(),
		newSSHCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
