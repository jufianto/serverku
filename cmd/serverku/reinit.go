package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/jufianto/serverku/internal/config"
	"github.com/spf13/cobra"
)

func newReinitCmd() *cobra.Command {
	var selected config.ProjectConfig
	var noStorage, nonInteractive bool
	cmd := &cobra.Command{
		Use: "reinit <project-name>", Short: "Rerun setup for an existing project, keeping other settings",
		Long:    "Rerun the setup wizard with current values prefilled. Notifications, routing, hooks, state and SSH keys are preserved. No cloud resources are changed. For scripts, --non-interactive changes only explicitly supplied flags.",
		Example: "  serverku reinit kuma\n  serverku reinit kuma --non-interactive --no-storage=false --storage-gb 10",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			path, err := store.ProjectPath(name)
			if err != nil {
				return err
			}
			original, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				return fmt.Errorf("project %q not found; run 'serverku init %s' first", name, name)
			}
			if err != nil {
				return err
			}
			before, err := config.ParseProjectDocument(name, original)
			if err != nil {
				return fmt.Errorf("%w; repair with 'serverku edit %s' first", err, name)
			}
			state, err := store.LoadState(name)
			if err != nil {
				return err
			}
			if state.VMID != "" || state.VMName != "" || state.IsRunning() || state.Status == config.StatusStopping {
				detail := ""
				if !before.Storage.Enabled {
					detail = " This project has no persistent disk: down deletes application data stored on the VM. Back it up first if needed."
				}
				return fmt.Errorf("project %q still tracks a VM; run 'serverku down %s' before reinit.%s", name, name, detail)
			}
			candidate := *before
			if nonInteractive {
				applyReinitFlags(cmd, &candidate, &selected, noStorage)
			} else {
				info, err := os.Stdin.Stat()
				if err != nil {
					return err
				}
				if info.Mode()&os.ModeCharDevice == 0 {
					return fmt.Errorf("reinit requires an interactive terminal; use --non-interactive with flags for scripting")
				}
				prompted, err := promptProjectConfig(&candidate)
				if err != nil {
					return err
				}
				candidate = *prompted
			}
			if candidate.Provider == "digitalocean" {
				candidate.ProjectID, candidate.Zone = "", ""
				if !cmd.Flags().Changed("spot") {
					candidate.VM.Spot = false
				}
			}
			if candidate.Provider != before.Provider && !cmd.Flags().Changed("image") {
				candidate.VM.Image = ""
				candidate.SetDefaults()
			}
			if err := candidate.Validate(); err != nil {
				return err
			}
			if err := config.ValidateProjectChange(before, &candidate, state); err != nil {
				return err
			}
			if err := validateInitCatalog(candidate.Provider, candidate.Region, candidate.VM.Size); err != nil {
				return err
			}
			edited, changes, err := config.MergeProjectSettings(original, before, &candidate)
			if err != nil {
				return err
			}
			if len(changes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Changes for %s:\n", name)
			for _, change := range changes {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", change)
			}
			if !nonInteractive {
				confirmed := false
				if err := huh.NewConfirm().Title("Save these configuration changes?").Value(&confirmed).Run(); err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintln(cmd.OutOrStdout(), "Canceled; original unchanged.")
					return nil
				}
			}
			if _, err := config.ParseProjectDocument(name, edited); err != nil {
				return err
			}
			backup, err := store.SaveProjectDocument(name, original, edited)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %s\nBackup: %s\nState and SSH keys preserved. Run 'serverku check %s', then 'serverku up %s' when ready.\n", path, backup, name, name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&selected.Provider, "provider", "p", "", "cloud provider (gcp, digitalocean)")
	cmd.Flags().StringVar(&selected.ProjectID, "project-id", "", "GCP project ID")
	cmd.Flags().StringVarP(&selected.Region, "region", "r", "", "cloud region")
	cmd.Flags().StringVarP(&selected.Zone, "zone", "z", "", "GCP zone")
	cmd.Flags().StringVarP(&selected.VM.Size, "size", "s", "", "VM machine type")
	cmd.Flags().StringVar(&selected.VM.Image, "image", "", "OS image")
	cmd.Flags().BoolVar(&selected.VM.Spot, "spot", false, "use GCP spot instances")
	cmd.Flags().BoolVar(&noStorage, "no-storage", false, "disable persistent storage (--no-storage=false enables it)")
	cmd.Flags().IntVar(&selected.Storage.SizeGB, "storage-gb", 20, "persistent disk size in GB")
	cmd.Flags().StringVar(&selected.Storage.MountPath, "mount-path", "/data", "persistent disk mount path")
	cmd.Flags().StringVar(&selected.ComposeFile, "compose-file", "", "Compose file path")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "change explicitly supplied flags without the wizard")
	return cmd
}

func applyReinitFlags(cmd *cobra.Command, candidate, selected *config.ProjectConfig, noStorage bool) {
	fields := map[string]struct {
		target *string
		value  string
	}{
		"provider": {&candidate.Provider, selected.Provider}, "project-id": {&candidate.ProjectID, selected.ProjectID},
		"region": {&candidate.Region, selected.Region}, "zone": {&candidate.Zone, selected.Zone},
		"size": {&candidate.VM.Size, selected.VM.Size}, "image": {&candidate.VM.Image, selected.VM.Image},
		"mount-path": {&candidate.Storage.MountPath, selected.Storage.MountPath}, "compose-file": {&candidate.ComposeFile, selected.ComposeFile},
	}
	for flag, field := range fields {
		if cmd.Flags().Changed(flag) {
			*field.target = field.value
		}
	}
	if cmd.Flags().Changed("spot") {
		candidate.VM.Spot = selected.VM.Spot
	}
	if cmd.Flags().Changed("no-storage") {
		candidate.Storage.Enabled = !noStorage
	}
	if cmd.Flags().Changed("storage-gb") {
		candidate.Storage.SizeGB = selected.Storage.SizeGB
	}
	if candidate.Storage.Enabled {
		if candidate.Storage.SizeGB == 0 {
			candidate.Storage.SizeGB = 20
		}
		if candidate.Storage.MountPath == "" {
			candidate.Storage.MountPath = "/data"
		}
	}
}
