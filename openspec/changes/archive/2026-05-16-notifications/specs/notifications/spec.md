## ADDED Requirements

### Requirement: Configuration for Notifications
The project configuration (`internal/config.ProjectConfig`) SHALL support a `Notifications` struct containing settings for Slack and Telegram.

#### Scenario: Slack Webhook Configured
- **WHEN** a project YAML file contains `notifications.slack.webhook_url`
- **THEN** the configuration parser SHALL successfully load this value.

#### Scenario: Telegram Bot Configured
- **WHEN** a project YAML file contains `notifications.telegram.bot_token` and `notifications.telegram.chat_id`
- **THEN** the configuration parser SHALL successfully load these values.

### Requirement: Slack Notifier
The system SHALL provide an `internal/notify/slack` package that implements the `Notifier` interface by sending HTTP POST requests to a Slack Incoming Webhook URL.

#### Scenario: Send Up Event to Slack
- **WHEN** `SendUp` is called on a Slack notifier
- **THEN** it SHALL send a message formatted for Slack indicating the project name is up and its IP address.

#### Scenario: Slack API Timeout
- **WHEN** the Slack API takes longer than 5 seconds to respond
- **THEN** the notifier SHALL return an error and not block indefinitely.

### Requirement: Telegram Notifier
The system SHALL provide an `internal/notify/telegram` package that implements the `Notifier` interface by sending HTTP POST requests to the Telegram Bot API (`sendMessage` endpoint).

#### Scenario: Send Error Event to Telegram
- **WHEN** `SendError` is called on a Telegram notifier
- **THEN** it SHALL send a message indicating the project name and the specific error encountered.

### Requirement: Multi-Notifier
The system SHALL provide a `MultiNotifier` that wraps multiple `Notifier` instances and calls them sequentially.

#### Scenario: Multiple Notifications Configured
- **WHEN** a `MultiNotifier` containing both Slack and Telegram notifiers receives a `SendUp` call
- **THEN** it SHALL call `SendUp` on both the Slack and Telegram notifiers.

### Requirement: Orchestrator Integration
The CLI entrypoint SHALL initialize the configured notifiers and pass them to the orchestrator. The orchestrator SHALL use the notifier for `Up`, `Down`, and `Error` events without failing the main operation if a notification fails.

#### Scenario: Notification Failure Does Not Abort Up
- **WHEN** the orchestrator calls `notifier.SendUp` and it returns an error
- **THEN** the orchestrator SHALL log the error but still return a successful `UpResult`.
