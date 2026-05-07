package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeStartURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "valid https", raw: "https://example.com", wantErr: false},
		{name: "missing scheme", raw: "example.com", wantErr: true},
		{name: "unsupported scheme", raw: "ftp://example.com", wantErr: true},
		{name: "empty host", raw: "https:///only-path", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := normalizeStartURL(tc.raw)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCrawlerDefaults(t *testing.T) {
	t.Parallel()

	c := NewCrawler(Config{})
	if c.config.MaxDepth != 2 {
		t.Fatalf("expected default depth=2, got %d", c.config.MaxDepth)
	}
	if c.config.HTTPTimeout != 30*time.Second {
		t.Fatalf("expected default timeout=30s, got %v", c.config.HTTPTimeout)
	}
	if c.config.RetryCount != 3 {
		t.Fatalf("expected default retries=3, got %d", c.config.RetryCount)
	}
	if c.config.RetryBackoff != 2*time.Second {
		t.Fatalf("expected default backoff=2s, got %v", c.config.RetryBackoff)
	}
	if c.config.BaseDelay != 2*time.Second {
		t.Fatalf("expected default base delay=2s, got %v", c.config.BaseDelay)
	}
}

func TestCrawlerCrawlSuccessDepthOne(t *testing.T) {
	t.Parallel()

	hits := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++

		switch r.URL.Path {
		case "/robots.txt":
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/a">a</a><a href="/b">b</a></body></html>`))
		case "/a", "/b":
			_, _ = w.Write([]byte(`<html><body>leaf</body></html>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	crawler := NewCrawler(Config{
		MaxDepth:     1,
		UserAgent:    "Grawl-Dynamic/1.0",
		HTTPTimeout:  3 * time.Second,
		RetryCount:   2,
		RetryBackoff: 5 * time.Millisecond,
		BaseDelay:    time.Millisecond,
	})

	res, err := crawler.Crawl(context.Background(), srv.URL+"/")
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	if res.Visited < 1 {
		t.Fatalf("expected at least one visited URL")
	}
	if res.Allowed < 1 {
		t.Fatalf("expected at least one allowed URL")
	}
	if res.Discovered < 2 {
		t.Fatalf("expected discovered links from root, got %d", res.Discovered)
	}
}

func TestCrawlerContextCancelled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}

		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`<html><body><a href="/next">n</a></body></html>`))
	}))
	defer srv.Close()

	crawler := NewCrawler(Config{MaxDepth: 3, HTTPTimeout: time.Second, RetryCount: 1, RetryBackoff: 10 * time.Millisecond, BaseDelay: time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := crawler.Crawl(ctx, srv.URL)
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
}

func TestCrawlerRobotsFetchError(t *testing.T) {
	t.Parallel()

	crawler := NewCrawler(Config{MaxDepth: 1, HTTPTimeout: time.Second, RetryCount: 1, RetryBackoff: time.Millisecond, BaseDelay: time.Millisecond})

	badURL := "http://127.0.0.1:1"
	_, err := crawler.Crawl(context.Background(), badURL)
	if err == nil {
		t.Fatalf("expected crawl error due to unreachable host")
	}

	if got := err.Error(); got == "" {
		t.Fatalf("expected non-empty error")
	}
}

func TestCrawlerRejectsBadStartURL(t *testing.T) {
	t.Parallel()

	crawler := NewCrawler(Config{})
	_, err := crawler.Crawl(context.Background(), "ftp://example.com")
	if err == nil {
		t.Fatalf("expected unsupported scheme error")
	}
}

func TestCrawlerHandlesRobotsDisallow(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /private\n"))
		case "/private":
			_, _ = w.Write([]byte(`<html><body><a href="/x">x</a></body></html>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	crawler := NewCrawler(Config{MaxDepth: 1, HTTPTimeout: time.Second, RetryCount: 1, RetryBackoff: time.Millisecond, BaseDelay: time.Millisecond})
	res, err := crawler.Crawl(context.Background(), srv.URL+"/private")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Allowed != 0 {
		t.Fatalf("expected zero allowed URLs under full disallow, got %d", res.Allowed)
	}
}

func ExampleCrawler_Crawl() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer srv.Close()

	crawler := NewCrawler(Config{MaxDepth: 1, HTTPTimeout: time.Second, RetryCount: 1, RetryBackoff: time.Millisecond, BaseDelay: time.Millisecond})
	result, err := crawler.Crawl(context.Background(), srv.URL)
	if err != nil {
		fmt.Println("error")
		return
	}

	fmt.Println(result.Allowed >= 1)
	// Output: true
}
