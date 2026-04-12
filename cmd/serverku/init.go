package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jufianto/serverku/internal/config"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var (
		provider  string
		projectID string
		region    string
		zone      string
		vmSize    string
		spot      bool
		noStorage bool
		storageGB int
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

			// Interactive mode if no provider flag given
			if !cmd.Flags().Changed("provider") {
				return runInitInteractive(name)
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

	return cmd
}

func runInitInteractive(name string) error {
	reader := bufio.NewReader(os.Stdin)

	// Provider
	fmt.Print("Cloud provider (gcp/digitalocean) [gcp]: ")
	provider, _ := reader.ReadString('\n')
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "gcp"
	}

	// Project ID (GCP only)
	var gcpProjectID string
	if provider == "gcp" {
		fmt.Print("GCP Project ID: ")
		gcpProjectID, _ = reader.ReadString('\n')
		gcpProjectID = strings.TrimSpace(gcpProjectID)
	}

	// Region
	defaultRegion := "asia-southeast1"
	if provider == "digitalocean" {
		defaultRegion = "sgp1"
	}
	fmt.Printf("Region [%s]: ", defaultRegion)
	region, _ := reader.ReadString('\n')
	region = strings.TrimSpace(region)
	if region == "" {
		region = defaultRegion
	}

	// Zone (GCP only)
	var zone string
	if provider == "gcp" {
		defaultZone := region + "-b"
		fmt.Printf("Zone [%s]: ", defaultZone)
		zone, _ = reader.ReadString('\n')
		zone = strings.TrimSpace(zone)
		if zone == "" {
			zone = defaultZone
		}
	}

	// VM size
	defaultSize := "e2-medium"
	if provider == "digitalocean" {
		defaultSize = "s-1vcpu-2gb"
	}
	fmt.Printf("VM size [%s]: ", defaultSize)
	vmSize, _ := reader.ReadString('\n')
	vmSize = strings.TrimSpace(vmSize)
	if vmSize == "" {
		vmSize = defaultSize
	}

	// SPOT
	fmt.Print("Use SPOT/preemptible instances? (y/n) [y]: ")
	spotStr, _ := reader.ReadString('\n')
	spotStr = strings.TrimSpace(strings.ToLower(spotStr))
	spot := spotStr == "" || spotStr == "y" || spotStr == "yes"

	// Storage
	fmt.Print("Enable persistent storage? (y/n) [y]: ")
	storageStr, _ := reader.ReadString('\n')
	storageStr = strings.TrimSpace(strings.ToLower(storageStr))
	storageEnabled := storageStr == "" || storageStr == "y" || storageStr == "yes"

	storageGB := 20
	mountPath := "/data"
	if storageEnabled {
		fmt.Print("Storage size in GB [20]: ")
		sizeStr, _ := reader.ReadString('\n')
		sizeStr = strings.TrimSpace(sizeStr)
		if sizeStr != "" {
			fmt.Sscanf(sizeStr, "%d", &storageGB)
		}

		fmt.Print("Mount path [/data]: ")
		mp, _ := reader.ReadString('\n')
		mp = strings.TrimSpace(mp)
		if mp != "" {
			mountPath = mp
		}
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
			SizeGB:    storageGB,
			MountPath: mountPath,
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

	fmt.Println()
	fmt.Printf("Project %q created at %s/projects/%s.yaml\n", name, store.BaseDir(), name)
	fmt.Println("Edit the config file to customize, then run: serverku up", name)
	return nil
}
