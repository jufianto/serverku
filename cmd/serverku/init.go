package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/jufianto/serverku/internal/config"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var (
		provider       string
		projectID      string
		region         string
		zone           string
		vmSize         string
		spot           bool
		noStorage      bool
		storageGB      int
		nonInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "init <project-name>",
		Short: "Create a new project configuration",
		Long: `Initialize a new serverku project. This creates a YAML configuration file
in ~/.serverku/projects/ that defines your VM, storage, and deployment settings.

You can pass flags for non-interactive setup, or run without flags for
interactive prompts.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Check if project already exists
			if store.ProjectExists(name) {
				return fmt.Errorf("project %q already exists", name)
			}

			// Interactive mode if we're in a TTY and non-interactive flag is not set
			isTerminal := false
			if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
				isTerminal = true
			}

			if isTerminal && !nonInteractive && !cmd.Flags().Changed("provider") {
				return runInteractiveInit(name)
			}

			// Non-interactive mode with flags
			cfg := &config.ProjectConfig{
				Name:      name,
				Provider:  provider,
				ProjectID: projectID,
				Region:    region,
				Zone:      zone,
				VM: config.VMConfig{
					Size: vmSize,
					Spot: spot,
				},
				Storage: config.StorageConfig{
					Enabled:   !noStorage,
					SizeGB:    storageGB,
					MountPath: "/data",
				},
			}

			cfg.SetDefaults()

			if err := cfg.Validate(); err != nil {
				return err
			}

			if err := store.SaveProject(cfg); err != nil {
				return err
			}

			// Ensure SSH keys exist
			_, _, err := store.EnsureSSHKeys()
			if err != nil {
				return fmt.Errorf("failed to generate SSH keys: %w", err)
			}

			fmt.Printf("Project %q created at %s/projects/%s.yaml\n", name, store.BaseDir(), name)
			fmt.Println("Edit the config file to customize, then run: serverku up", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "cloud provider (gcp, digitalocean)")
	cmd.Flags().StringVar(&projectID, "project-id", "", "cloud project ID (required for GCP)")
	cmd.Flags().StringVarP(&region, "region", "r", "", "cloud region")
	cmd.Flags().StringVarP(&zone, "zone", "z", "", "cloud zone (required for GCP)")
	cmd.Flags().StringVarP(&vmSize, "size", "s", "e2-medium", "VM machine type")
	cmd.Flags().BoolVar(&spot, "spot", true, "use SPOT/preemptible instances")
	cmd.Flags().BoolVar(&noStorage, "no-storage", false, "create a fully stateless project (no persistent disk)")
	cmd.Flags().IntVar(&storageGB, "storage-gb", 20, "persistent disk size in GB")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "skip interactive prompts")

	return cmd
}

func runInteractiveInit(name string) error {
	var (
		provider       string
		gcpProjectID   string
		region         string
		zone           string
		vmSize         string
		spot           bool
		storageEnabled bool
		storageGB      string
		mountPath      string
		composeFile    string
	)

	// Defaults that might change based on provider
	defaultRegion := "asia-southeast1"
	defaultZone := "asia-southeast1-b"
	defaultSize := "e2-medium"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Cloud Provider").
				Options(
					huh.NewOption("GCP", "gcp"),
					huh.NewOption("DigitalOcean", "digitalocean"),
				).
				Value(&provider),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("GCP Project ID").
				Value(&gcpProjectID).
				Validate(func(s string) error {
					if provider == "gcp" && s == "" {
						return fmt.Errorf("project ID is required for GCP")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return provider != "gcp" }),
		huh.NewGroup(
			huh.NewInput().
				Title("Region").
				Value(&region).
				DescriptionFunc(func() string {
					if provider == "digitalocean" {
						return "e.g., sgp1, nyc1"
					}
					return "e.g., asia-southeast1, us-central1"
				}, &provider),
			huh.NewInput().
				Title("Zone (GCP only)").
				Value(&zone).
				DescriptionFunc(func() string {
					if region != "" {
						return fmt.Sprintf("e.g., %s-b", region)
					}
					return "e.g., asia-southeast1-b"
				}, &region),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("VM Size").
				Value(&vmSize).
				DescriptionFunc(func() string {
					if provider == "digitalocean" {
						return "e.g., s-1vcpu-1gb"
					}
					return "e.g., e2-medium"
				}, &provider),
			huh.NewConfirm().
				Title("Use SPOT / Preemptible instances?").
				Value(&spot),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable persistent storage?").
				Value(&storageEnabled),
			huh.NewInput().
				Title("Storage Size (GB)").
				Value(&storageGB).
				Validate(func(s string) error {
					if storageEnabled {
						var size int
						if _, err := fmt.Sscanf(s, "%d", &size); err != nil || size <= 0 {
							return fmt.Errorf("must be a positive integer")
						}
					}
					return nil
				}),
			huh.NewInput().
				Title("Mount Path").
				Value(&mountPath).
				Validate(func(s string) error {
					if storageEnabled && s == "" {
						return fmt.Errorf("mount path is required")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return !storageEnabled }),
		huh.NewGroup(
			huh.NewInput().
				Title("Compose File Path").
				Value(&composeFile),
		),
	)

	// Set initial values so user doesn't have to type them if they are ok with defaults
	region = defaultRegion
	zone = defaultZone
	vmSize = defaultSize
	spot = true
	storageEnabled = true
	storageGB = "20"
	mountPath = "/data"
	composeFile = "docker-compose.yml"

	err := form.Run()
	if err != nil {
		return err
	}

	// Process answers
	if provider == "digitalocean" {
		if region == defaultRegion {
			region = "sgp1"
		} // fallback if left as default GCP region
		if vmSize == defaultSize {
			vmSize = "s-1vcpu-1gb"
		}
	}

	sizeGB := 20
	if storageEnabled {
		_, _ = fmt.Sscanf(storageGB, "%d", &sizeGB)
	}

	cfg := &config.ProjectConfig{
		Name:      name,
		Provider:  provider,
		ProjectID: gcpProjectID,
		Region:    region,
		Zone:      zone,
		VM: config.VMConfig{
			Size: vmSize,
			Spot: spot,
		},
		Storage: config.StorageConfig{
			Enabled:   storageEnabled,
			SizeGB:    sizeGB,
			MountPath: mountPath,
		},
		ComposeFile: composeFile,
	}

	cfg.SetDefaults()

	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := store.SaveProject(cfg); err != nil {
		return err
	}

	// Ensure SSH keys exist
	_, _, err = store.EnsureSSHKeys()
	if err != nil {
		return fmt.Errorf("failed to generate SSH keys: %w", err)
	}

	fmt.Println()
	fmt.Printf("Project %q created at %s/projects/%s.yaml\n", name, store.BaseDir(), name)
	fmt.Println("Edit the config file to customize, then run: serverku up", name)
	return nil
}
