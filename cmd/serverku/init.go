package main

import (
	"fmt"
	"os"
	"strconv"

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
		sshKey         string
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
				return fmt.Errorf("project %q already exists; use 'serverku edit %s' or 'serverku reinit %s'", name, name, name)
			}

			// Interactive mode if we're in a TTY and non-interactive flag is not set
			isTerminal := false
			if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
				isTerminal = true
			}

			if isTerminal && !nonInteractive && !cmd.Flags().Changed("provider") {
				return runInteractiveInit(name, sshKey)
			}

			// A project ID belongs to GCP; never persist it for DigitalOcean.
			if provider == "digitalocean" {
				projectID = ""
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
				SSH:       config.SSHConfig{PrivateKey: sshKey},
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

			// Ensure SSH keys exist
			_, err = store.ResolveProjectSSHKey(cfg, nil, true)
			if err != nil {
				return fmt.Errorf("failed to generate SSH keys: %w", err)
			}
			if err := store.SaveProject(cfg); err != nil {
				return err
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
	cmd.Flags().StringVar(&sshKey, "ssh-key", "", "existing unencrypted SSH private key (default: generate a project key)")

	return cmd
}

func runInteractiveInit(name string, sshKey string) error {
	initial := &config.ProjectConfig{Name: name, VM: config.VMConfig{Spot: true},
		SSH:     config.SSHConfig{PrivateKey: sshKey},
		Storage: config.StorageConfig{Enabled: true, SizeGB: 20, MountPath: "/data"}, ComposeFile: "docker-compose.yml"}
	cfg, err := promptProjectConfig(initial)
	if err != nil {
		return err
	}
	topic, err := config.GenerateNtfyTopic(name)
	if err != nil {
		return err
	}
	cfg.Notifications.Ntfy.Topic = topic
	if _, err := store.ResolveProjectSSHKey(cfg, nil, true); err != nil {
		return fmt.Errorf("failed to generate SSH keys: %w", err)
	}
	if err := store.SaveProject(cfg); err != nil {
		return err
	}
	fmt.Printf("Project %q created at %s/projects/%s.yaml\n", name, store.BaseDir(), name)
	fmt.Printf("Notifications: run `serverku ntfy %s` to set up push notifications\n", name)
	fmt.Printf("Edit with 'serverku edit %s', then run 'serverku up %s'.\n", name, name)
	return nil
}

func promptProjectConfig(initial *config.ProjectConfig) (*config.ProjectConfig, error) {
	providerName, projectID := initial.Provider, initial.ProjectID
	region, zone, size := initial.Region, initial.Zone, initial.VM.Size
	spot, storage := initial.VM.Spot, initial.Storage.Enabled
	storageGB := fmt.Sprintf("%d", initial.Storage.SizeGB)
	mount, compose := initial.Storage.MountPath, initial.ComposeFile
	if projectID == "" {
		projectID = defaultGCPProject()
	}

	// Separate dependent stages so hidden GCP/storage questions are omitted
	// in both the terminal UI and huh's accessible (plain-text) mode.
	if err := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title("Cloud Provider").
		Options(huh.NewOption("DigitalOcean", "digitalocean"), huh.NewOption("GCP", "gcp")).Value(&providerName))).Run(); err != nil {
		return nil, err
	}
	if providerName != initial.Provider && initial.Provider != "" {
		region, zone, size = "", "", ""
		spot = providerName == "gcp"
	}
	if providerName == "gcp" {
		if err := huh.NewForm(huh.NewGroup(huh.NewInput().Title("GCP Project ID").Value(&projectID).
			Validate(func(value string) error {
				if value == "" {
					value = projectID
				}
				if value == "" {
					return fmt.Errorf("project ID is required for GCP")
				}
				return nil
			}))).Run(); err != nil {
			return nil, err
		}
	} else {
		projectID, zone, spot = "", "", false
	}

	cat := loadCatalog(providerName)
	if cat.live {
		fmt.Printf("Loaded live regions and sizes from %s.\n", providerName)
	} else {
		fmt.Printf("Using built-in %s size/region suggestions (edit the YAML for anything not listed).\n", providerName)
	}
	if err := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title("Region").
		Options(withCurrentOption(cat.regionOptions(), region)...).Value(&region))).Run(); err != nil {
		return nil, err
	}
	var groups []*huh.Group
	if providerName == "gcp" {
		groups = append(groups, huh.NewGroup(huh.NewInput().Title("Zone").Value(&zone).
			Description(fmt.Sprintf("e.g., %s-b", region)).
			Validate(func(value string) error {
				if value == "" {
					value = zone
				}
				if value == "" {
					return fmt.Errorf("zone is required for GCP")
				}
				return nil
			})))
	}
	sizeOptions := cat.sizeOptions(region)
	if region == initial.Region && providerName == initial.Provider {
		sizeOptions = withCurrentOption(sizeOptions, size)
	}
	groups = append(groups, huh.NewGroup(huh.NewSelect[string]().Title("VM Size").Options(sizeOptions...).Value(&size)))
	if providerName == "gcp" {
		groups = append(groups, huh.NewGroup(huh.NewConfirm().Title("Use SPOT / Preemptible instances?").Value(&spot)))
	}
	groups = append(groups, huh.NewGroup(huh.NewConfirm().Title("Enable persistent storage?").Value(&storage)))
	if err := huh.NewForm(groups...).Run(); err != nil {
		return nil, err
	}

	groups = nil
	if storage {
		if initial.Storage.SizeGB == 0 {
			storageGB = "20"
		}
		if mount == "" {
			mount = "/data"
		}
		groups = append(groups, huh.NewGroup(huh.NewInput().Title("Storage Size (GB)").Value(&storageGB).
			Validate(func(value string) error {
				if value == "" {
					value = storageGB
				}
				gb, err := strconv.Atoi(value)
				if err != nil || gb <= 0 {
					return fmt.Errorf("must be a positive integer")
				}
				return nil
			}),
			huh.NewInput().Title("Mount Path").Value(&mount).
				Validate(func(value string) error {
					if value == "" {
						value = mount
					}
					if value == "" {
						return fmt.Errorf("mount path is required")
					}
					return nil
				})))
	}
	groups = append(groups, huh.NewGroup(huh.NewInput().Title("Compose File Path").Value(&compose)))
	if err := huh.NewForm(groups...).Run(); err != nil {
		return nil, err
	}

	candidate := *initial
	candidate.Provider, candidate.ProjectID, candidate.Region, candidate.Zone = providerName, projectID, region, zone
	candidate.VM.Size, candidate.VM.Spot = size, spot
	candidate.Storage.Enabled, candidate.Storage.MountPath = storage, mount
	if storage {
		candidate.Storage.SizeGB, _ = strconv.Atoi(storageGB)
	}
	candidate.ComposeFile = compose
	if providerName != initial.Provider && initial.Provider != "" {
		candidate.VM.Image = ""
	}
	candidate.SetDefaults()
	if err := candidate.Validate(); err != nil {
		return nil, err
	}
	return &candidate, nil
}

// Keep a saved selection available when the live or fallback catalog omits it.
func withCurrentOption(options []huh.Option[string], current string) []huh.Option[string] {
	if current == "" {
		return options
	}
	for _, option := range options {
		if option.Value == current {
			return options
		}
	}
	return append([]huh.Option[string]{huh.NewOption(current+" (current)", current)}, options...)
}
