package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/orchestrator"
	"github.com/jufianto/serverku/internal/pricing"
	"github.com/jufianto/serverku/internal/provider"
	"github.com/jufianto/serverku/internal/provisioner"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status <project-name>",
		Short:   "Show the current status of a project",
		Example: "  serverku status kuma\n  serverku list",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("provide exactly one project name\nUsage: %s\nExample: serverku status kuma\nRun 'serverku list' to find your project name", cmd.UseLine())
			}
			return nil
		},
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

			// Live inventory of the cloud resources serverku manages for this
			// project, so it's clear what exists and what destroy leaves behind.
			printComponents(cmd.Context(), cfg, state, factory)

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

// printComponents shows a live inventory of the cloud resources serverku
// manages for the project, flagging any that `serverku destroy` does not remove
// (orphans the user must clean up manually). It is best-effort: if the provider
// can't be built or doesn't support the ComponentLister capability, it prints
// nothing rather than failing the status command.
func printComponents(ctx context.Context, cfg *config.ProjectConfig, state *config.ProjectState, factory orchestrator.ProviderFactory) {
	cp, err := factory(ctx, cfg)
	if err != nil {
		return
	}
	lister, ok := cp.(provider.ComponentLister)
	if !ok {
		return
	}

	// Derive resource names from state when available, else from the naming
	// convention, so the inventory works even before/after the VM exists.
	vmName := state.VMName
	if vmName == "" {
		vmName = "serverku-" + cfg.Name
	}
	diskName := state.DiskName
	if diskName == "" && cfg.Storage.Enabled {
		diskName = "serverku-" + cfg.Name + "-data"
	}

	pubKey, _ := store.GetSSHPublicKey()
	comps, err := lister.ListComponents(ctx, provider.ComponentQuery{
		ProjectName: cfg.Name,
		VMName:      vmName,
		DiskName:    diskName,
		SSHPubKey:   pubKey,
	})
	if err != nil || len(comps) == 0 {
		return
	}

	fmt.Println("\nComponents (live):")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	orphans := 0
	for _, c := range comps {
		name := c.Name
		if c.Detail != "" {
			if name != "" {
				name += "  "
			}
			name += "(" + c.Detail + ")"
		}
		if name == "" {
			name = "-"
		}

		var st string
		switch {
		case c.LookupFailed:
			st = "unknown"
		case c.Present && c.Shared:
			st = "present  shared (retained by destroy)"
		case c.Present && !c.RemovedByDestroy:
			st = "present  ⚠ orphan (destroy won't remove)"
			orphans++
		case c.Present:
			st = "present"
		default:
			st = "none"
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\n", c.Kind, name, st)
	}
	_ = w.Flush()

	if orphans > 0 {
		fmt.Printf("\n  ⚠ %d component(s) are NOT removed by `serverku destroy` -- delete manually.\n", orphans)
	}
}
