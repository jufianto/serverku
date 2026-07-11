package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/notify"
	"github.com/jufianto/serverku/internal/notify/ntfy"
	"github.com/jufianto/serverku/internal/notify/slack"
	"github.com/jufianto/serverku/internal/notify/telegram"
	"github.com/jufianto/serverku/internal/orchestrator"
	"github.com/jufianto/serverku/internal/pricing"
	"github.com/jufianto/serverku/internal/provider"
	"github.com/jufianto/serverku/internal/provider/digitalocean"
	"github.com/jufianto/serverku/internal/provider/gcp"
	"github.com/jufianto/serverku/internal/provisioner"
	"github.com/spf13/cobra"
)

// newProviderFactory returns a ProviderFactory that creates the appropriate
// cloud provider based on the project config.
func newProviderFactory() orchestrator.ProviderFactory {
	return func(ctx context.Context, cfg *config.ProjectConfig) (provider.CloudProvider, error) {
		switch cfg.Provider {
		case "gcp":
			return gcp.New(ctx, cfg.ProjectID, cfg.Zone)
		case "digitalocean":
			return digitalocean.New(ctx)
		default:
			return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
		}
	}
}

// buildNotifier constructs a MultiNotifier based on the project configuration.
func buildNotifier(cfg *config.ProjectConfig) notify.Notifier {
	var notifiers []notify.Notifier

	if cfg.Notifications.Slack.WebhookURL != "" {
		notifiers = append(notifiers, slack.New(cfg.Notifications.Slack.WebhookURL))
	}

	if cfg.Notifications.Telegram.BotToken != "" && cfg.Notifications.Telegram.ChatID != "" {
		notifiers = append(notifiers, telegram.New(cfg.Notifications.Telegram.BotToken, cfg.Notifications.Telegram.ChatID))
	}

	if cfg.Notifications.Ntfy.Topic != "" {
		notifiers = append(notifiers, ntfy.New(cfg.Notifications.Ntfy.ServerURL(), cfg.Notifications.Ntfy.Topic))
	}

	if len(notifiers) == 0 {
		return &notify.NoopNotifier{}
	}
	return notify.NewMultiNotifier(notifiers...)
}

func newUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up <project-name>",
		Short: "Spin up a project's VM and deploy containers",
		Long: `Create a VM, attach persistent storage (if enabled), install Docker,
and deploy your docker-compose stack. The server will be accessible via
the external IP shown on completion.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Verify project exists
			cfg, err := store.LoadProject(name)
			if err != nil {
				return err
			}

			fmt.Printf("Starting project %q...\n", name)
			fmt.Printf("  Provider: %s\n", cfg.Provider)
			fmt.Printf("  Region:   %s / %s\n", cfg.Region, cfg.Zone)
			fmt.Printf("  VM:       %s (spot: %v)\n", cfg.VM.Size, cfg.VM.Spot)
			if cfg.Storage.Enabled {
				fmt.Printf("  Storage:  %dGB at %s\n", cfg.Storage.SizeGB, cfg.Storage.MountPath)
			}
			fmt.Println()

			notifier := buildNotifier(cfg)
			prov := &provisioner.SSHProvisioner{}
			orch := orchestrator.New(store, prov, notifier, nil)
			factory := newProviderFactory()

			result, err := orch.Up(cmd.Context(), name, factory)
			if err != nil {
				return fmt.Errorf("failed to start project: %w", err)
			}

			fmt.Println()
			fmt.Printf("Project %q is running!\n", name)
			fmt.Printf("  External IP: %s\n", result.ExternalIP)
			fmt.Printf("  VM:          %s (id: %s)\n", result.VMName, result.VMID)
			if result.DiskName != "" {
				fmt.Printf("  Disk:        %s (id: %s)\n", result.DiskName, result.DiskID)
			}
			rate := newRateCache(factory).resolveRate(cmd.Context(), cfg)
			for _, line := range pricing.FormatUpEstimate(pricing.EstimateCost(cfg), rate) {
				fmt.Println(line)
			}
			fmt.Println()
			fmt.Printf("SSH: ssh serverku@%s\n", result.ExternalIP)
			fmt.Printf("Stop: serverku down %s\n", name)

			return nil
		},
	}
}

func newDeployCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deploy <project-name>",
		Short: "Push code changes to a running VM without recreating it",
		Long: `Re-sync the project directory, rewrite the compose file, and run
docker compose up -d on the already-running VM. Same IP, no DNS churn,
seconds instead of minutes. The project must be running (see: serverku up).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := store.LoadProject(name)
			if err != nil {
				return err
			}

			fmt.Printf("Deploying project %q...\n", name)

			notifier := buildNotifier(cfg)
			orch := orchestrator.New(store, &provisioner.SSHProvisioner{}, notifier, nil)

			if err := orch.Deploy(cmd.Context(), name); err != nil {
				return err
			}

			fmt.Printf("Project %q deployed.\n", name)
			fmt.Printf("Logs: serverku logs %s\n", name)
			return nil
		},
	}
}

func newDownCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "down <project-name>",
		Short: "Tear down a project's VM, keep persistent storage",
		Long: `Stop containers, detach persistent storage, and destroy the VM.
Your data on the persistent disk is preserved for the next 'serverku up'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := store.LoadProject(name)
			if err != nil {
				return err
			}

			if !force {
				fmt.Printf("This will destroy the VM for project %q. Data on persistent storage is preserved.\n", name)
				fmt.Print("Continue? (y/n): ")
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if confirm != "y" && confirm != "yes" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			fmt.Printf("Stopping project %q...\n", name)

			notifier := buildNotifier(cfg)
			prov := &provisioner.SSHProvisioner{}
			orch := orchestrator.New(store, prov, notifier, nil)
			factory := newProviderFactory()

			if err := orch.Down(cmd.Context(), name, factory); err != nil {
				return fmt.Errorf("failed to stop project: %w", err)
			}

			fmt.Printf("Project %q stopped. Persistent storage preserved.\n", name)
			fmt.Printf("Restart: serverku up %s\n", name)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	return cmd
}

func newDestroyCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "destroy <project-name>",
		Short: "Permanently delete a project and all its resources",
		Long: `Destroy everything: VM, persistent storage, and project configuration.
This action is irreversible - all data will be permanently lost.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := store.LoadProject(name)
			if err != nil {
				return err
			}

			if !force {
				fmt.Printf("WARNING: This will permanently delete ALL resources for project %q.\n", name)
				fmt.Printf("This includes the VM, persistent storage, and all data.\n")
				fmt.Printf("Type the project name to confirm: ")
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if confirm != name {
					fmt.Println("Aborted. Name did not match.")
					return nil
				}
			}

			fmt.Printf("Destroying project %q...\n", name)

			notifier := buildNotifier(cfg)
			prov := &provisioner.SSHProvisioner{}
			orch := orchestrator.New(store, prov, notifier, nil)
			factory := newProviderFactory()

			if err := orch.Destroy(cmd.Context(), name, factory); err != nil {
				return fmt.Errorf("failed to destroy project: %w", err)
			}

			fmt.Printf("Project %q destroyed. All resources have been permanently deleted.\n", name)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	return cmd
}

func init() {
	// Suppress log timestamp prefix for cleaner output when not verbose
	log.SetFlags(0)
}
