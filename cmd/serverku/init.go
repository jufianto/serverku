package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/provider/gcp"
	"github.com/spf13/cobra"
)

// defaultGCPProject returns the project chosen during `serverku setup gcp`
// (recorded as the quota project in serverku's isolated ADC), or "" when setup
// has not run. It lets init pre-fill the GCP project ID.
func defaultGCPProject() string {
	if store == nil {
		return ""
	}
	return gcp.QuotaProjectFromADC(store.GcloudADCPath())
}

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

			// The --spot default (true) only makes sense for GCP; DigitalOcean
			// has no spot equivalent. Unless the user explicitly asked for
			// spot, default it off for DigitalOcean instead of writing a
			// config that fails at `up`.
			if provider == "digitalocean" && !cmd.Flags().Changed("spot") {
				spot = false
			}

			// Default the GCP project to the one chosen in `serverku setup gcp`
			// so a configured user doesn't have to repeat it.
			if provider == "gcp" && !cmd.Flags().Changed("project-id") {
				if p := defaultGCPProject(); p != "" {
					projectID = p
					fmt.Printf("Using GCP project %q from `serverku setup gcp` (override with --project-id).\n", projectID)
				}
			}

			// Catch an invalid --region/--size against the live catalog now,
			// with suggestions, instead of a cryptic 422 at create time.
			if err := validateInitCatalog(provider, region, vmSize); err != nil {
				return err
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

			// Generate an unguessable ntfy topic so push notifications work
			// out of the box (see `serverku ntfy <project>`).
			topic, err := config.GenerateNtfyTopic(name)
			if err != nil {
				return err
			}
			cfg.Notifications.Ntfy.Topic = topic

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

			fmt.Printf("Project %q created at %s/projects/%s.yaml\n", name, store.BaseDir(), name)
			fmt.Printf("Notifications: run `serverku ntfy %s` to set up push notifications\n", name)
			fmt.Println("Edit the config file to customize, then run: serverku up", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "cloud provider (gcp, digitalocean)")
	cmd.Flags().StringVar(&projectID, "project-id", "", "cloud project ID (required for GCP)")
	cmd.Flags().StringVarP(&region, "region", "r", "", "cloud region")
	cmd.Flags().StringVarP(&zone, "zone", "z", "", "cloud zone (required for GCP)")
	cmd.Flags().StringVarP(&vmSize, "size", "s", "e2-medium", "VM machine type")
	cmd.Flags().BoolVar(&spot, "spot", true, "use SPOT/preemptible instances (GCP only; defaults to false for digitalocean)")
	cmd.Flags().BoolVar(&noStorage, "no-storage", false, "create a fully stateless project (no persistent disk)")
	cmd.Flags().IntVar(&storageGB, "storage-gb", 20, "persistent disk size in GB")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "skip interactive prompts")

	return cmd
}

func runInteractiveInit(name string) error {
	var (
		providerName   string
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

	// Pre-fill the GCP project from `serverku setup gcp`, if it ran.
	gcpProjectID = defaultGCPProject()

	// Stage 1: pick the provider (and GCP project) first, so we know which
	// region/size catalog to load before asking for them.
	stage1 := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Cloud Provider").
				Options(
					huh.NewOption("DigitalOcean", "digitalocean"),
					huh.NewOption("GCP", "gcp"),
				).
				Value(&providerName),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("GCP Project ID").
				Value(&gcpProjectID).
				Validate(func(s string) error {
					if providerName == "gcp" && s == "" {
						return fmt.Errorf("project ID is required for GCP")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return providerName != "gcp" }),
	)
	if err := stage1.Run(); err != nil {
		return err
	}

	// Load the region/size catalog: live from the provider API when possible,
	// otherwise a built-in curated list.
	cat := loadCatalog(providerName)
	if cat.live {
		fmt.Printf("Loaded live regions and sizes from %s.\n", providerName)
	} else {
		fmt.Printf("Using built-in %s size/region suggestions (edit the YAML for anything not listed).\n", providerName)
	}

	// Sensible defaults for the non-catalog fields.
	spot = true
	storageEnabled = true
	storageGB = "20"
	mountPath = "/data"
	composeFile = "docker-compose.yml"

	// Stage 2: region/size come from the catalog (selectable, not free text);
	// the size list reacts to the chosen region via OptionsFunc.
	stage2 := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Region").
				Options(cat.regionOptions()...).
				Value(&region),
			huh.NewInput().
				Title("Zone (GCP only)").
				Value(&zone).
				DescriptionFunc(func() string {
					if region != "" {
						return fmt.Sprintf("e.g., %s-b", region)
					}
					return "e.g., asia-southeast1-b"
				}, &region).
				Validate(func(s string) error {
					if providerName == "gcp" && s == "" {
						return fmt.Errorf("zone is required for GCP")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("VM Size").
				OptionsFunc(func() []huh.Option[string] {
					return cat.sizeOptions(region)
				}, &region).
				Value(&vmSize),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Use SPOT / Preemptible instances?").
				Value(&spot),
		).WithHideFunc(func() bool { return providerName != "gcp" }),
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
	if err := stage2.Run(); err != nil {
		return err
	}

	// The spot question is hidden for DigitalOcean (no spot equivalent), but its
	// default value is true -- reset it so the config validates.
	if providerName == "digitalocean" {
		spot = false
	}

	sizeGB := 20
	if storageEnabled {
		_, _ = fmt.Sscanf(storageGB, "%d", &sizeGB)
	}

	cfg := &config.ProjectConfig{
		Name:      name,
		Provider:  providerName,
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

	// Generate an unguessable ntfy topic so push notifications work out of
	// the box (see `serverku ntfy <project>`).
	topic, err := config.GenerateNtfyTopic(name)
	if err != nil {
		return err
	}
	cfg.Notifications.Ntfy.Topic = topic

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
	fmt.Printf("Notifications: run `serverku ntfy %s` to set up push notifications\n", name)
	fmt.Println("Edit the config file to customize, then run: serverku up", name)
	return nil
}
