package main

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/jufianto/serverku/internal/config"
	"github.com/jufianto/serverku/internal/notify/ntfy"
	"github.com/jufianto/serverku/internal/notify/slack"
	"github.com/jufianto/serverku/internal/notify/telegram"
	"github.com/spf13/cobra"
)

// chatDiscoveryTimeout bounds how long `notify setup` waits for the user to
// message their Telegram bot during chat_id auto-discovery.
const chatDiscoveryTimeout = 2 * time.Minute

func newNotifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Set up and test notification channels",
	}
	cmd.AddCommand(newNotifySetupCmd(), newNotifyTestCmd())
	return cmd
}

func newNotifySetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup <project-name>",
		Short: "Interactive wizard: connect a notification channel and verify it end to end",
		Long: `Guides you through connecting ntfy (account-less push, recommended) or
Telegram to a project, verifies the connection by sending a real test
notification from this machine, and optionally enables the on-VM
still-running heartbeat.

For Telegram, the wizard verifies your bot token and auto-discovers the
chat_id: you just send your bot one message.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := store.LoadProject(name)
			if err != nil {
				return err
			}

			var channel string
			if err := huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Which channel do you want to set up?").
					Options(
						huh.NewOption("ntfy — push app, no account needed (recommended)", "ntfy"),
						huh.NewOption("Telegram — bot messages to a chat", "telegram"),
					).
					Value(&channel),
			)).Run(); err != nil {
				return err
			}

			switch channel {
			case "ntfy":
				err = setupNtfy(cmd.Context(), cfg)
			case "telegram":
				err = setupTelegram(cmd.Context(), cfg)
			}
			if err != nil {
				return err
			}

			if err := store.SaveProject(cfg); err != nil {
				return err
			}
			fmt.Printf("\nSaved to %s/projects/%s.yaml\n", store.BaseDir(), cfg.Name)
			fmt.Printf("Re-test anytime with: serverku notify test %s\n", cfg.Name)
			return nil
		},
	}
}

// setupNtfy walks the user through subscribing to the project's topic and
// proves the pipe works with a real test notification.
func setupNtfy(ctx context.Context, cfg *config.ProjectConfig) error {
	if cfg.Notifications.Ntfy.Topic == "" {
		topic, err := config.GenerateNtfyTopic(cfg.Name)
		if err != nil {
			return err
		}
		cfg.Notifications.Ntfy.Topic = topic
	}
	nt := cfg.Notifications.Ntfy
	server := nt.ServerURL()

	fmt.Println()
	fmt.Println("Step 1 — Subscribe on your device (one-time):")
	fmt.Println("  iPhone:  App Store 'ntfy' -> + -> Subscribe to topic")
	fmt.Println("  Android: Play Store / F-Droid 'ntfy' -> + -> Subscribe to topic")
	fmt.Printf("  Topic:   %s\n", nt.Topic)
	fmt.Printf("  Browser alternative: open %s/%s\n\n", server, nt.Topic)

	ready := true
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Subscribed and ready for a test notification?").Value(&ready),
	)).Run(); err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("setup cancelled -- run `serverku notify setup %s` again when ready", cfg.Name)
	}

	fmt.Println("Step 2 — Sending a test notification from this machine...")
	if err := ntfy.New(server, nt.Topic).SendTest(ctx, cfg.Name); err != nil {
		return fmt.Errorf("test notification failed: %w", err)
	}

	received := true
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Did the notification arrive on your device?").Value(&received),
	)).Run(); err != nil {
		return err
	}
	if !received {
		fmt.Println("Troubleshooting: check the topic name matches exactly, and that")
		fmt.Println("notifications are allowed for the ntfy app in system settings.")
		return fmt.Errorf("test notification not received")
	}

	return askHeartbeat(&cfg.Notifications.Ntfy.HeartbeatHours)
}

// setupTelegram verifies the bot token, auto-discovers the chat_id from the
// user's first message to the bot, and proves the pipe with a test message.
func setupTelegram(ctx context.Context, cfg *config.ProjectConfig) error {
	fmt.Println()
	fmt.Println("Step 1 — Create a bot (skip if you have one):")
	fmt.Println("  In Telegram, message @BotFather -> /newbot -> copy the token.")
	fmt.Println()

	token := cfg.Notifications.Telegram.BotToken
	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Bot token").Value(&token).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("token is required")
				}
				return nil
			}),
	)).Run(); err != nil {
		return err
	}

	fmt.Println("Verifying token with Telegram...")
	bot, err := telegram.GetMe(ctx, token)
	if err != nil {
		return fmt.Errorf("token verification failed: %w", err)
	}
	fmt.Printf("Token OK — bot is @%s\n\n", bot.Username)

	fmt.Printf("Step 2 — Open https://t.me/%s and send the bot any message.\n", bot.Username)
	fmt.Printf("Waiting for your message (up to %s)...\n", chatDiscoveryTimeout)

	discoverCtx, cancel := context.WithTimeout(ctx, chatDiscoveryTimeout)
	defer cancel()
	chat, err := telegram.DiscoverChatID(discoverCtx, token)
	if err != nil {
		return fmt.Errorf("could not discover chat: %w (did you message @%s?)", err, bot.Username)
	}
	fmt.Printf("Found chat with %s (chat_id %s)\n\n", chat.Name, chat.ID)

	fmt.Println("Step 3 — Sending a test notification from this machine...")
	if err := telegram.New(token, chat.ID).SendTest(ctx, cfg.Name); err != nil {
		return fmt.Errorf("test notification failed: %w", err)
	}

	received := true
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Did the bot's test message arrive in Telegram?").Value(&received),
	)).Run(); err != nil {
		return err
	}
	if !received {
		return fmt.Errorf("test notification not received")
	}

	cfg.Notifications.Telegram.BotToken = token
	cfg.Notifications.Telegram.ChatID = chat.ID

	fmt.Println()
	fmt.Println("Note: enabling the heartbeat places this bot token (root-only) on the")
	fmt.Println("VM. Prefer the ntfy channel if you want no account-linked secret there.")
	return askHeartbeat(&cfg.Notifications.Telegram.HeartbeatHours)
}

// askHeartbeat optionally enables the on-VM still-running reminder.
func askHeartbeat(hours *int) error {
	enable := *hours > 0
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Enable the still-running heartbeat (reminder every 6h while the VM is up)?").
			Value(&enable),
	)).Run(); err != nil {
		return err
	}
	if enable && *hours == 0 {
		*hours = 6
	}
	if !enable {
		*hours = 0
	}
	return nil
}

func newNotifyTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <project-name>",
		Short: "Send a test notification to every configured channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := store.LoadProject(name)
			if err != nil {
				return err
			}

			type channel struct {
				name string
				send func(context.Context, string) error
			}
			var channels []channel

			if nt := cfg.Notifications.Ntfy; nt.Topic != "" {
				channels = append(channels, channel{"ntfy", ntfy.New(nt.ServerURL(), nt.Topic).SendTest})
			}
			if tg := cfg.Notifications.Telegram; tg.BotToken != "" && tg.ChatID != "" {
				channels = append(channels, channel{"telegram", telegram.New(tg.BotToken, tg.ChatID).SendTest})
			}
			if sl := cfg.Notifications.Slack; sl.WebhookURL != "" {
				channels = append(channels, channel{"slack", slack.New(sl.WebhookURL).SendTest})
			}

			if len(channels) == 0 {
				return fmt.Errorf("no notification channels configured for %q -- run: serverku notify setup %s", name, name)
			}

			failures := 0
			for _, ch := range channels {
				if err := ch.send(cmd.Context(), name); err != nil {
					failures++
					fmt.Printf("  ✗ %-8s %v\n", ch.name, err)
					continue
				}
				fmt.Printf("  ✓ %-8s test notification sent\n", ch.name)
			}

			if failures > 0 {
				return fmt.Errorf("%d of %d channels failed", failures, len(channels))
			}
			fmt.Println("\nAll channels OK.")
			return nil
		},
	}
}
