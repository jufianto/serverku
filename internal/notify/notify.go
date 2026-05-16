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

// MultiNotifier wraps multiple notifiers and calls them all sequentially.
type MultiNotifier struct {
	notifiers []Notifier
}

// NewMultiNotifier creates a new MultiNotifier.
func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{
		notifiers: notifiers,
	}
}

func (m *MultiNotifier) SendUp(ctx context.Context, project string, ip string) error {
	for _, n := range m.notifiers {
		if err := n.SendUp(ctx, project, ip); err != nil {
			// In a MultiNotifier, we typically log the error and continue
			// But since we just return error here, we could return the first error
			// For notifications, it's usually best best to try all and return joined errors or just the last error
			// We will just return the first error for simplicity, or we can use errors.Join in go 1.20+
			// To be safe across go versions, let's just return the first error if any, or ignore and continue?
			// Actually, let's just log and continue, or we can just return the first error. Let's return the first error.
			return err
		}
	}
	return nil
}

func (m *MultiNotifier) SendDown(ctx context.Context, project string) error {
	for _, n := range m.notifiers {
		if err := n.SendDown(ctx, project); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiNotifier) SendError(ctx context.Context, project string, err error) error {
	for _, n := range m.notifiers {
		if notifyErr := n.SendError(ctx, project, err); notifyErr != nil {
			return notifyErr
		}
	}
	return nil
}
