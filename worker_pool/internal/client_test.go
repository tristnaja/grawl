package internal

import (
	"testing"
	"time"
)

func TestNewClientDefaults(t *testing.T) {
	client := NewClient()

	if client == nil {
		t.Fatalf("expected client instance")
	}

	if client.BotName != "Grawl" {
		t.Fatalf("unexpected bot name: %s", client.BotName)
	}

	if client.AgentName == "" {
		t.Fatalf("expected non-empty agent name")
	}

	if client.Client == nil {
		t.Fatalf("expected http client")
	}

	if client.Client.Timeout != 30*time.Second {
		t.Fatalf("unexpected timeout: %v", client.Client.Timeout)
	}

	client.Context.cancel()
	select {
	case <-client.Context.ctx.Done():
	default:
		t.Fatalf("expected context cancellation")
	}
}
