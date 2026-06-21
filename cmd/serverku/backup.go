package main

import (
	"fmt"
	"time"

	"github.com/jufianto/serverku/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newBackupCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "backup <project-name>",
		Short: "Snapshot a project's persistent disk",
		Long: `Create a snapshot of the project's persistent storage volume via the
cloud provider. The VM may be up or down; only the disk is required.

Use --name to set a custom snapshot name, otherwise one is generated as
serverku-<project>-<timestamp>.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]

			snapshotName := name
			if snapshotName == "" {
				snapshotName = fmt.Sprintf("serverku-%s-%s", project, time.Now().Format("20060102-150405"))
			}

			fmt.Printf("Snapshotting project %q as %q...\n", project, snapshotName)

			orch := orchestrator.New(store, nil, nil, nil)
			factory := newProviderFactory()

			result, err := orch.Backup(cmd.Context(), project, snapshotName, factory)
			if err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}

			fmt.Printf("Snapshot created: %s (id: %s) from disk %s\n", result.SnapshotName, result.SnapshotID, result.DiskName)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "custom snapshot name (default: serverku-<project>-<timestamp>)")
	return cmd
}
