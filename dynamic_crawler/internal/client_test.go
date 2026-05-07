package internal

import (
	"testing"
	"time"
)

func TestNewHTTPClientTimeout(t *testing.T) {
	client := newHTTPClient(5 * time.Second)
	if client == nil {
		t.Fatalf("expected http client")
	}

	if client.Timeout != 5*time.Second {
		t.Fatalf("unexpected timeout: %v", client.Timeout)
	}
}
