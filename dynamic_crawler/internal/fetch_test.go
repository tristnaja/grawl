package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestFetchDocumentSuccess(t *testing.T) {
	t.Parallel()

	seenUA := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	doc, err := fetchDocument(context.Background(), srv.Client(), "Dyn/1.0", parsed, 2, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("fetchDocument failed: %v", err)
	}

	if doc == nil {
		t.Fatalf("expected parsed doc")
	}

	if seenUA != "Dyn/1.0" {
		t.Fatalf("unexpected user-agent: %s", seenUA)
	}
}

func TestFetchDocumentClientErrorNoRetry(t *testing.T) {
	t.Parallel()

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	_, err = fetchDocument(context.Background(), srv.Client(), "Dyn/1.0", parsed, 3, 10*time.Millisecond)
	if err == nil {
		t.Fatalf("expected non-retryable error")
	}

	if hits != 1 {
		t.Fatalf("expected single attempt for 4xx, got %d", hits)
	}
}

func TestFetchDocumentRetryThenSuccess(t *testing.T) {
	t.Parallel()

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	_, err = fetchDocument(context.Background(), srv.Client(), "Dyn/1.0", parsed, 4, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}

	if hits != 3 {
		t.Fatalf("expected 3 attempts, got %d", hits)
	}
}

func TestFetchDocumentContextCancel(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = fetchDocument(ctx, srv.Client(), "Dyn/1.0", parsed, 5, 100*time.Millisecond)
	if err == nil {
		t.Fatalf("expected context cancellation")
	}
}

func TestCollectLinksDedupAndSchemeFilter(t *testing.T) {
	t.Parallel()

	doc, err := html.Parse(strings.NewReader(`<html><body>
		<a href="/a">a</a>
		<a href="/a">a2</a>
		<a href="mailto:test@example.com">mail</a>
		<a href="https://example.com/b">b</a>
	</body></html>`))
	if err != nil {
		t.Fatalf("parse html failed: %v", err)
	}

	base, err := url.Parse("https://example.com/root")
	if err != nil {
		t.Fatalf("parse base failed: %v", err)
	}

	links := collectLinks(doc, base)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d (%v)", len(links), links)
	}
}
