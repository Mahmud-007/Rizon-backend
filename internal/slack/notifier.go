package slack

import "context"

// Notifier defines the interface for publishing messages to a notification channel.
// This abstraction allows swapping mock with real Slack integration without refactoring.
type Notifier interface {
	Publish(ctx context.Context, message string) error
}
