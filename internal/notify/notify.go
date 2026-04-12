package notify

import "context"

// Notifier defines the interface for sending project lifecycle notifications.
type Notifier interface {
	// SendUp sends a notification that a project's VM is up and running.
	SendUp(ctx context.Context, project string, ip string) error

	// SendDown sends a notification that a project's VM has been shut down.
	SendDown(ctx context.Context, project string) error

	// SendError sends a notification about an error with a project.
	SendError(ctx context.Context, project string, err error) error
}

// NoopNotifier is a notifier that does nothing.
// Used when notifications are not configured.
type NoopNotifier struct{}

func (n *NoopNotifier) SendUp(_ context.Context, _ string, _ string) error   { return nil }
func (n *NoopNotifier) SendDown(_ context.Context, _ string) error           { return nil }
func (n *NoopNotifier) SendError(_ context.Context, _ string, _ error) error { return nil }
