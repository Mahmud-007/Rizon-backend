package slack

import (
	"context"
	"log"
)

// MockSlack implements the Notifier interface by logging messages to stdout.
// Replace this with a real Slack client for production use.
type MockSlack struct{}

func NewMockSlack() *MockSlack {
	return &MockSlack{}
}

func (m *MockSlack) Publish(ctx context.Context, message string) error {
	log.Printf("📨 [MockSlack] Published to Slack channel: %s", message)
	return nil
}
