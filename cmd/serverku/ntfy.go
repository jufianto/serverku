package main

import (
	"fmt"

	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/notify/ntfy"
	"github.com/spf13/cobra"
)

func newNtfyCmd() *cobra.Command {
	var test bool

	cmd := &cobra.Command{
		Use:   "ntfy <project-name>",
		Short: "Show how to subscribe to a project's push notifications (and send a test)",
		Long: `ntfy (https://ntfy.sh) delivers serverku's push notifications without any
account or token: the project's randomly generated topic name is the only
thing you need. This command shows the topic, how to subscribe on your
phone or browser, and can send a test notification with --test.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := store.LoadProject(name)
			if err != nil {
				return err
			}

			// Older projects may predate topic generation -- create one now.
			if cfg.Notifications.Ntfy.Topic == "" {
				topic, err := config.GenerateNtfyTopic(name)
				if err != nil {
					return err
				}
				cfg.Notifications.Ntfy.Topic = topic
				if err := store.SaveProject(cfg); err != nil {
					return err
				}
				fmt.Printf("Generated ntfy topic and saved it to the project config.\n\n")
			}

			nt := cfg.Notifications.Ntfy
			server := nt.ServerURL()

			if test {
				fmt.Printf("Sending test notification to %s/%s ...\n", server, nt.Topic)
				if err := ntfy.New(server, nt.Topic).SendTest(cmd.Context(), name); err != nil {
					return fmt.Errorf("test notification failed: %w", err)
				}
				fmt.Println("Sent! It should appear on every subscribed device within seconds.")
				fmt.Println("Not seeing it? Subscribe first -- run this command without --test for instructions.")
				return nil
			}

			fmt.Printf("Push notifications for project %q\n\n", name)
			fmt.Printf("  Server: %s\n", server)
			fmt.Printf("  Topic:  %s\n\n", nt.Topic)
			fmt.Println("The topic name is the only secret: anyone who knows it can send")
			fmt.Println("notifications to it, so share it like a password.")
			fmt.Println()
			fmt.Println("Subscribe (one-time, ~2 minutes):")
			fmt.Println("  iPhone:  App Store 'ntfy' -> + -> Subscribe to topic -> paste the topic")
			fmt.Println("  Android: Play Store / F-Droid 'ntfy' -> + -> paste the topic")
			fmt.Printf("  Browser: open %s/%s\n", server, nt.Topic)
			fmt.Println()
			fmt.Println("Then verify it works:")
			fmt.Printf("  serverku ntfy %s --test\n", name)
			fmt.Println()
			fmt.Println("What you'll receive:")
			fmt.Println("  - up/down/error notifications when you run serverku commands")
			if nt.HeartbeatHours > 0 {
				fmt.Printf("  - a still-running reminder from the VM every %dh (heartbeat enabled)\n", nt.HeartbeatHours)
			} else {
				fmt.Println("  - optional: a still-running reminder from the VM while it is up.")
				fmt.Printf("    Enable it in the project config:\n\n")
				fmt.Println("      notifications:")
				fmt.Println("        ntfy:")
				fmt.Printf("          topic: %s\n", nt.Topic)
				fmt.Println("          heartbeat_hours: 6")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&test, "test", false, "send a test notification to the project's topic")

	return cmd
}
