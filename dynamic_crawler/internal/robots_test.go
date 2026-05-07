package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestRobotsAllowWithAllowDisallow(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		_, _ = w.Write([]byte("User-agent: Grawl-Dynamic/1.0\nDisallow: /private\nAllow: /private/open\n"))
	}))
	defer srv.Close()

	store := newRobotsStore(context.Background(), srv.Client(), time.Millisecond)

	ok, err := store.allow("Grawl-Dynamic/1.0", srv.URL+"/private")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected /private blocked")
	}

	ok, err = store.allow("Grawl-Dynamic/1.0", srv.URL+"/private/open")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected /private/open allowed")
	}
}

func TestRobotsAllowInvalidURL(t *testing.T) {
	t.Parallel()

	store := newRobotsStore(context.Background(), http.DefaultClient, time.Millisecond)
	if _, err := store.allow("any", "://bad"); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestRobotsEnsureFailure(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("offline")
	})}

	store := newRobotsStore(context.Background(), client, time.Millisecond)
	if _, err := store.allow("agent", "https://example.com"); err == nil {
		t.Fatalf("expected error from failed robots fetch")
	}
}

func TestRobotsWaitReusesTicker(t *testing.T) {
	t.Parallel()

	store := newRobotsStore(context.Background(), http.DefaultClient, 10*time.Millisecond)
	parsed, err := url.Parse("https://example.com/path")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	firstDone := make(chan struct{})
	go func() {
		store.wait(parsed)
		close(firstDone)
	}()

	select {
	case <-firstDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("wait timed out")
	}

	store.mu.Lock()
	firstTicker := store.limit[parsed.Host]
	store.mu.Unlock()

	secondDone := make(chan struct{})
	go func() {
		store.wait(parsed)
		close(secondDone)
	}()

	select {
	case <-secondDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("second wait timed out")
	}

	store.mu.Lock()
	secondTicker := store.limit[parsed.Host]
	store.mu.Unlock()

	if firstTicker != secondTicker {
		t.Fatalf("expected ticker reuse for same host")
	}

	firstTicker.Stop()
}

func TestNewRobotsStoreDefaultDelay(t *testing.T) {
	t.Parallel()

	store := newRobotsStore(context.Background(), http.DefaultClient, 0)
	if store.baseDelay != 2*time.Second {
		t.Fatalf("expected default base delay 2s, got %v", store.baseDelay)
	}
}

func TestRobotsEnsureScannerError(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(failingReader{}),
			Header:     make(http.Header),
		}, nil
	})}

	store := newRobotsStore(context.Background(), client, time.Millisecond)
	_, err := store.allow("agent", "https://example.com")
	if err == nil {
		t.Fatalf("expected scanner read error")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("forced read failure")
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
