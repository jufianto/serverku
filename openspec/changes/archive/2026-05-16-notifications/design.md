## Context

The `serverku` CLI currently has an `internal/notify` package that defines a `Notifier` interface, but the only implementation is `NoopNotifier`. The orchestrator already takes a `notify.Notifier` interface in its constructor and attempts to call `SendUp` and `SendDown` during lifecycle events, but since it's currently hardcoded to the no-op implementation or ignored, users receive no alerts.

To provide actual notifications, we need to implement concrete notifiers (starting with Slack and Telegram, as they are the most common), add configuration options for them in the project `yaml` files, and wire them up in the main CLI entrypoint.

## Goals / Non-Goals

**Goals:**
- Add `Notifications` configuration struct to `ProjectConfig` to hold webhook URLs and tokens.
- Implement `internal/notify/slack` supporting incoming webhooks.
- Implement `internal/notify/telegram` supporting bot API messages to specific chat IDs.
- Ensure the Orchestrator successfully uses the configured notifier(s) on `Up`, `Down`, and when it encounters an error.

**Non-Goals:**
- Implement a generic email notifier (SMTP can be complex to configure correctly; we'll stick to webhooks/HTTP APIs for now).
- Support interactive notifications (e.g., clicking a button in Slack to approve a deployment). This is purely one-way alerting.

## Decisions

**1. Configuration Structure:**
We will add a `Notifications` struct to `ProjectConfig`.
```yaml
notifications:
  slack:
    webhook_url: "https://hooks.slack.com/services/..."
  telegram:
    bot_token: "1234:ABCDEF"
    chat_id: "-100123456"
```
- *Rationale*: Keeps configuration centralized per-project.

**2. Multi-Notifier approach:**
A project might want both Slack and Telegram notifications. We will implement a `MultiNotifier` (a composite pattern) in the `notify` package that iterates through a slice of active notifiers and calls them all.
- *Rationale*: Provides maximum flexibility for the user.

**3. Error Handling in Notifications:**
Notification failures (e.g., Slack API is down) MUST NOT fail the overall `serverku up` or `serverku down` operation.
- *Rationale*: Notifications are a secondary concern. The infrastructure state is the primary concern. The orchestrator will log the error but proceed.

## Risks / Trade-offs

- **[Risk] Exposing sensitive tokens in YAML** → *Mitigation*: The `serverku` config files live in `~/.serverku/projects/` which should be read/write only by the user, similar to SSH keys. In the future, we could support reading these from environment variables.
- **[Risk] Long timeouts on webhook calls blocking the orchestrator** → *Mitigation*: The HTTP clients inside the notifiers should have strict, short timeouts (e.g., 5 seconds) to ensure they don't block the CLI execution significantly.
