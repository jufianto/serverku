## Why

Currently, `serverku` performs infrastructure lifecycle operations (`up`, `down`, `destroy`) but does not inform users of the results asynchronously. If a user runs `serverku up` in a CI/CD pipeline or closes their terminal, they don't get an alert when the deployment finishes or if it fails. Implementing a notification system allows users to receive real-time alerts in their preferred communication channels (like Slack, Discord, or Telegram).

## What Changes

- Update project configuration to support a `Notifications` section (e.g., Slack webhook URL, Telegram bot token/chat ID).
- Implement concrete `Notifier` structures (e.g., `SlackNotifier`, `TelegramNotifier`) in the `internal/notify` package, fulfilling the existing `Notifier` interface.
- Wire the configured notifier into the `orchestrator` so that `SendUp`, `SendDown`, and `SendError` are called at the appropriate lifecycle events.

## Capabilities

### New Capabilities
- `notifications`: Support for sending lifecycle event notifications (Up, Down, Error) to external messaging platforms like Slack and Telegram.

### Modified Capabilities
- `<none>`

## Impact

- **Code:** Modifies `internal/config` to add notification settings. Implements new files in `internal/notify`. Modifies `cmd/serverku` to initialize the correct notifier and pass it to the orchestrator.
- **Dependencies:** May require lightweight HTTP clients or specific SDKs for Slack/Telegram, though standard library `net/http` is often sufficient for webhooks.
- **Systems:** External systems will start receiving webhooks/API calls when lifecycle events occur.
