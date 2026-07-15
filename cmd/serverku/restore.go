package main

import (
	"fmt"
	"time"

	"github.com/jufianto/serverku/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newRestoreCmd() *cobra.Command {
	var (
		diskName  string
		deleteOld bool
	)

	cmd := &cobra.Command{
		Use:   "restore <project-name> <snapshot>",
		Short: "Restore a project's disk from a snapshot",
		Long: `Create a new persistent disk from a snapshot (by name or ID, as printed by
serverku backup) and point the project at it. The project must be stopped
(run serverku down first); the next serverku up attaches the restored disk.

The previous disk is kept by default so you can roll back; pass --delete-old
to remove it (and stop paying for it) once you trust the restore.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, snapshot := args[0], args[1]

			newName := diskName
			if newName == "" {
				newName = fmt.Sprintf("serverku-%s-data-%s", project, time.Now().Format("20060102-150405"))
			}

			fmt.Printf("Restoring project %q from snapshot %q into disk %q...\n", project, snapshot, newName)

			orch := orchestrator.New(store, nil, nil, nil)
			factory := newProviderFactory()

			result, err := orch.Restore(cmd.Context(), project, snapshot, newName, deleteOld, factory)
			if err != nil {
				return fmt.Errorf("restore failed: %w", err)
			}

			fmt.Printf("Restored: project %q now uses disk %s (id: %s)\n", project, result.NewDiskName, result.NewDiskID)
			switch {
			case result.OldDeleted:
				fmt.Printf("Old disk %s deleted.\n", result.OldDiskName)
			case result.OldDiskName != "":
				fmt.Printf("Old disk %s kept (delete it manually or re-run with --delete-old).\n", result.OldDiskName)
			}
			fmt.Printf("Bring it online with: serverku up %s\n", project)
			return nil
		},
	}

	cmd.Flags().StringVar(&diskName, "name", "", "name for the restored disk (default: serverku-<project>-data-<timestamp>)")
	cmd.Flags().BoolVar(&deleteOld, "delete-old", false, "delete the previous disk after restoring")
	return cmd
}
