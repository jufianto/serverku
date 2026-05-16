## 1. Configuration Setup

- [x] 1.1 Add `Notifications` struct to `internal/config/config.go` (`ProjectConfig`).
- [x] 1.2 Add `Slack` config struct (`WebhookURL`).
- [x] 1.3 Add `Telegram` config struct (`BotToken`, `ChatID`).

## 2. Notifier Core Implementation

- [x] 2.1 Update `internal/notify/notify.go` to add `MultiNotifier` implementation.
- [x] 2.2 Create `internal/notify/slack/slack.go`.
- [x] 2.3 Implement `SlackNotifier` implementing `notify.Notifier` interface (Up, Down, Error methods via HTTP POST).
- [x] 2.4 Create `internal/notify/telegram/telegram.go`.
- [x] 2.5 Implement `TelegramNotifier` implementing `notify.Notifier` interface (Up, Down, Error methods via HTTP POST to Bot API).
- [x] 2.6 Add HTTP timeout (e.g. 5 seconds) to clients in both Slack and Telegram notifiers to prevent blocking.

## 3. Orchestrator Integration

- [x] 3.1 Update `cmd/serverku/lifecycle.go` (`Up` command) to initialize configured notifiers based on `cfg.Notifications` and create a `MultiNotifier`.
- [x] 3.2 Pass the initialized notifier to `orchestrator.New(...)`.
- [x] 3.3 Ensure the orchestrator's `setErrorState` or relevant error handling paths call `notifier.SendError`.

## 4. Testing

- [x] 4.1 Add tests for `MultiNotifier` in `notify_test.go`.
- [x] 4.2 (Optional/Manual) Verify Slack and Telegram notifications fire correctly when running `serverku up` and `serverku down`.
