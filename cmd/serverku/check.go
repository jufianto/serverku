package main

import (
	"fmt"

	"github.com/jufianto/serverku/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <project-name>",
		Short: "Verify a project is ready to deploy, without creating anything",
		Long: `Run preflight checks before spending money: validate the config, confirm
the compose file and sync directory exist, ensure the SSH keypair is
present, verify the cloud credentials actually authenticate, and confirm
DNS support when dns.enabled. No cloud resources are created.

Handy as a pre_up hook so up refuses to run when something is wrong.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			orch := orchestrator.New(store, nil, nil, nil)
			factory := newProviderFactory()

			result, err := orch.Preflight(cmd.Context(), name, factory)
			if err != nil {
				return err
			}

			for _, c := range result.Checks {
				mark := "✓"
				if !c.OK {
					mark = "✗"
				}
				line := fmt.Sprintf("  %s %-13s", mark, c.Name)
				if c.Detail != "" {
					line += " " + c.Detail
				}
				fmt.Println(line)
			}

			if !result.AllOK() {
				return fmt.Errorf("preflight checks failed for %q", name)
			}
			fmt.Printf("\nProject %q is ready. Bring it up with: serverku up %s\n", name, name)
			return nil
		},
	}
}
