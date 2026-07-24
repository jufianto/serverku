package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/orchestrator"
	"github.com/jufianto/serverku/internal/pricing"
	"github.com/jufianto/serverku/internal/provisioner"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <project-name>",
		Short: "Show the current status of a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := store.LoadProject(name)
			if err != nil {
				return err
			}

			// Use orch.Status() so SPOT termination detection works -- it
			// reconciles local state against the cloud provider.
			orch := orchestrator.New(store, &provisioner.SSHProvisioner{}, nil, nil)
			factory := newProviderFactory()

			state, err := orch.Status(cmd.Context(), name, factory)
			if err != nil {
				return err
			}

			rates := newRateCache(factory)
			printProjectStatus(cfg, state, rates.resolveRate(cmd.Context(), cfg))

			// Real account-level month-to-date usage, when the provider's API
			// reports it (currently DigitalOcean).
			if usd, ok := rates.monthToDateUsage(cmd.Context(), cfg); ok {
				fmt.Printf("\nAccount month-to-date (%s): $%.2f\n", cfg.Provider, usd)
			}
			return nil
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects and their status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := store.ListProjects()
			if err != nil {
				return err
			}

			if len(names) == 0 {
				fmt.Println("No projects found. Run 'serverku init <name>' to create one.")
				return nil
			}

			rates := newRateCache(newProviderFactory())

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tPROVIDER\tSTATUS\tIP\tSTORAGE\tCOST")
			fmt.Fprintln(w, "----\t--------\t------\t--\t-------\t----")

			for _, name := range names {
				cfg, err := store.LoadProject(name)
				if err != nil {
					fmt.Fprintf(w, "%s\t-\terror\t-\t-\t-\n", name)
					continue
				}

				state, err := store.LoadState(name)
				if err != nil {
					fmt.Fprintf(w, "%s\t%s\terror\t-\t-\t-\n", name, cfg.Provider)
					continue
				}

				ip := "-"
				if state.ExternalIP != "" {
					ip = state.ExternalIP
				}

				storage := "disabled"
				if cfg.Storage.Enabled {
					storage = fmt.Sprintf("%dGB", cfg.Storage.SizeGB)
				}

				// Running projects show accrued session cost at the best
				// available rate (live API price, or table estimate marked
				// est.); stopped projects show the storage-only estimate.
				cost := pricing.FormatListEstimate(pricing.EstimateCost(cfg), state.IsRunning())
				if state.IsRunning() && state.StartedAt != nil {
					rate := rates.resolveRate(cmd.Context(), cfg)
					if rate.Known {
						cost = pricing.FormatAccrued(rate, *state.StartedAt, time.Now())
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					name, cfg.Provider, state.Status, ip, storage, cost)
			}

			return w.Flush()
		},
	}
}

func printProjectStatus(cfg *config.ProjectConfig, state *config.ProjectState, rate pricing.Rate) {
	fmt.Printf("Project:   %s\n", cfg.Name)
	fmt.Printf("Provider:  %s\n", cfg.Provider)
	fmt.Printf("Region:    %s\n", cfg.Region)
	if cfg.Zone != "" {
		fmt.Printf("Zone:      %s\n", cfg.Zone)
	}
	fmt.Printf("VM Size:   %s\n", cfg.VM.Size)
	// Spot is a GCP-only concept; DigitalOcean has no equivalent, so don't
	// show a misleading "Spot: false" for providers that don't support it.
	if providerSupportsSpot(cfg.Provider) {
		fmt.Printf("Spot:      %v\n", cfg.VM.Spot)
	}
	fmt.Printf("Storage:   ")
	if cfg.Storage.Enabled {
		fmt.Printf("%dGB at %s\n", cfg.Storage.SizeGB, cfg.Storage.MountPath)
	} else {
		fmt.Println("disabled (stateless)")
	}
	fmt.Println()
	fmt.Printf("Status:    %s\n", state.Status)

	if state.ExternalIP != "" {
		fmt.Printf("IP:        %s\n", state.ExternalIP)
	}
	if state.VMID != "" {
		fmt.Printf("VM ID:     %s\n", state.VMID)
	}
	if state.DiskID != "" {
		fmt.Printf("Disk ID:   %s\n", state.DiskID)
	}
	if state.StartedAt != nil {
		uptime := time.Since(*state.StartedAt).Truncate(time.Second)
		fmt.Printf("Uptime:    %s\n", uptime)
		fmt.Printf("Started:   %s\n", state.StartedAt.Format(time.RFC3339))
		if state.IsRunning() {
			// Accrued compute cost for this session: uptime x hourly rate.
			// Live rates come from the provider's pricing API; table rates
			// are marked est. so an estimate is never mistaken for a bill.
			fmt.Printf("Session:   %s\n", pricing.FormatAccrued(rate, *state.StartedAt, time.Now()))
		}
	}
	if state.ErrorMsg != "" {
		fmt.Printf("Error:     %s\n", state.ErrorMsg)
	}
}
