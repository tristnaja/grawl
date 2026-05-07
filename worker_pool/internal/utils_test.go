package internal

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestFetchPageSuccess(t *testing.T) {
	t.Parallel()

	seenUserAgent := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUserAgent = r.Header.Get("User-Agent")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body><a href='/a'>a</a></body></html>"))
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("url parse failed: %v", err)
	}

	doc, err := FetchPage(context.Background(), srv.Client(), "Agent/1.0", parsed)
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}

	if doc == nil {
		t.Fatalf("expected non-nil document")
	}

	if seenUserAgent != "Agent/1.0" {
		t.Fatalf("unexpected user agent: %s", seenUserAgent)
	}
}

func TestFetchPageClientErrorNoRetry(t *testing.T) {
	t.Parallel()

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("url parse failed: %v", err)
	}

	_, err = FetchPage(context.Background(), srv.Client(), "Agent/1.0", parsed)
	if err == nil {
		t.Fatalf("expected error for 404")
	}

	if hits != 1 {
		t.Fatalf("expected single request for client error, got %d", hits)
	}
}

func TestFetchPageContextCancelled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("url parse failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = FetchPage(ctx, srv.Client(), "Agent/1.0", parsed)
	if err == nil {
		t.Fatalf("expected cancellation error")
	}

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected fast cancellation, took %v", elapsed)
	}
}

func TestFetchPageInvalidHTMLReturnsParseError(t *testing.T) {
	t.Parallel()

	parsed, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatalf("url parse failed: %v", err)
	}

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(errorReader{}),
			Header:     make(http.Header),
		}, nil
	})}

	_, err = FetchPage(context.Background(), client, "Agent/1.0", parsed)
	if err == nil {
		t.Fatalf("expected parse error for unreadable response body")
	}
}

func TestTraverseNodeResolvesAndDedupesLinks(t *testing.T) {
	t.Parallel()

	root, err := html.Parse(strings.NewReader(`
<html><body>
	<a href="/x">x</a>
	<a href="/x">x2</a>
	<a href="https://example.com/y">y</a>
</body></html>`))
	if err != nil {
		t.Fatalf("html parse failed: %v", err)
	}

	base, err := url.Parse("https://example.com/base")
	if err != nil {
		t.Fatalf("base parse failed: %v", err)
	}

	res := Result{StartURL: base, Finding: make([]string, 0)}
	TraverseNode(root, &res)

	if len(res.Finding) != 2 {
		t.Fatalf("expected 2 unique links, got %d (%v)", len(res.Finding), res.Finding)
	}
}
