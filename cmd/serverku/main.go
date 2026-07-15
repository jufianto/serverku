package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/jufianto/serverku/internal/config"
	"github.com/spf13/cobra"
)

// Build metadata. version is set by goreleaser via -ldflags on tagged
// releases; commit and date are stamped there too. For local/dev builds these
// stay empty and resolveVersion() falls back to the Go build info embedded by
// `go build` (VCS revision, commit time, dirty flag).
var (
	version = "dev"
	commit  = ""
	date    = ""
)

var (
	configDir string
	verbose   bool
	store     *config.Store
)

// resolveVersion assembles the user-facing version string, preferring
// ldflags-injected values and filling gaps from the embedded build info.
func resolveVersion() string {
	v, c, d := version, commit, date
	dirty := false

	if info, ok := debug.ReadBuildInfo(); ok {
		hasVCS := false
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if c == "" {
					c = s.Value
				}
				hasVCS = true
			case "vcs.time":
				if d == "" {
					d = s.Value
				}
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		// Fall back to the module version only when there is no VCS stamp
		// (e.g. `go install pkg@vX.Y.Z` from the module cache). From a source
		// checkout the vcs.* settings above are cleaner than a pseudo-version.
		if v == "dev" && !hasVCS && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}

	return formatVersion(v, c, d, dirty)
}

// formatVersion renders the version string from its parts:
// "1.2.3 (abc123def456-dirty, 2026-07-15T...)". Empty parts are omitted.
func formatVersion(version, commit, date string, dirty bool) string {
	var extra []string
	switch {
	case commit != "":
		short := commit
		if len(short) > 12 {
			short = short[:12]
		}
		if dirty {
			short += "-dirty"
		}
		extra = append(extra, short)
	case dirty:
		extra = append(extra, "dirty")
	}
	if date != "" {
		extra = append(extra, date)
	}
	if len(extra) == 0 {
		return version
	}
	return version + " (" + strings.Join(extra, ", ") + ")"
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "serverku",
		Short: "On-demand cloud VM orchestrator",
		Long: `serverku creates and destroys cloud VMs on demand.
Define a project, keep the server off. When you need it, one command spins up
a VM, attaches persistent storage, and deploys your Docker stack.
When done, tear it down. Pay only for storage when idle.`,
		Version: resolveVersion(),
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
		newSetupCmd(),
		newCheckCmd(),
		newUpCmd(),
		newDeployCmd(),
		newDownCmd(),
		newStatusCmd(),
		newListCmd(),
		newDestroyCmd(),
		newSSHCmd(),
		newOpenCmd(),
		newLogsCmd(),
		newTunnelCmd(),
		newBackupCmd(),
		newRestoreCmd(),
		newNtfyCmd(),
		newNotifyCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
