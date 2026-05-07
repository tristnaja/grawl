package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRobotsIsAllowedWithRules(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		_, _ = w.Write([]byte("User-agent: Grawl\nDisallow: /private\nAllow: /private/open\n"))
	}))
	defer srv.Close()

	robot := NewRobot(context.Background(), srv.Client())

	blocked, err := robot.IsAllowed("Grawl", srv.URL+"/private")
	if err != nil {
		t.Fatalf("unexpected error for blocked path: %v", err)
	}
	if blocked {
		t.Fatalf("expected /private to be blocked")
	}

	allowed, err := robot.IsAllowed("Grawl", srv.URL+"/private/open")
	if err != nil {
		t.Fatalf("unexpected error for allowed path: %v", err)
	}
	if !allowed {
		t.Fatalf("expected /private/open to be allowed")
	}
}

func TestRobotsRateLimitReusesTickerPerHost(t *testing.T) {
	t.Parallel()

	robot := NewRobot(context.Background(), http.DefaultClient)
	parsed, err := url.Parse("https://example.com/path")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	first := robot.RateLimit("Grawl", parsed)
	second := robot.RateLimit("Grawl", parsed)

	if first != second {
		t.Fatalf("expected same ticker for same host")
	}

	first.Stop()
}

func TestFetchRulesInvalidURL(t *testing.T) {
	t.Parallel()

	robot := NewRobot(context.Background(), http.DefaultClient)
	if err := robot.FetchRules("://not-a-url"); err == nil {
		t.Fatalf("expected invalid URL error")
	}
}

func TestRobotsFetchFailureReturnsErrorButAllowedDefault(t *testing.T) {
	t.Parallel()

	robot := NewRobot(context.Background(), &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network down")
	})})

	allowed, err := robot.IsAllowed("Grawl", "https://example.com/")
	if err == nil {
		t.Fatalf("expected error when fetch fails")
	}
	if !allowed {
		t.Fatalf("expected allowed=true fallback when robots fetch fails")
	}
}

func TestRobotsFetchRulesScannerError(t *testing.T) {
	t.Parallel()

	robot := NewRobot(context.Background(), &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(errorReader{}),
			Header:     make(http.Header),
		}, nil
	})})

	err := robot.FetchRules("https://example.com")
	if err != nil {
		t.Fatalf("expected no scanner error handling in current implementation, got: %v", err)
	}
}

func TestRobotsFetchRulesCachesPerHost(t *testing.T) {
	t.Parallel()

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.Copy(w, strings.NewReader("User-agent: *\nAllow: /\n"))
	}))
	defer srv.Close()

	robot := NewRobot(context.Background(), srv.Client())
	if err := robot.FetchRules(srv.URL + "/a"); err != nil {
		t.Fatalf("fetch rules failed: %v", err)
	}
	if err := robot.FetchRules(srv.URL + "/b"); err != nil {
		t.Fatalf("second fetch rules failed: %v", err)
	}

	if hits != 1 {
		t.Fatalf("expected one robots fetch due to cache, got %d", hits)
	}
}

func TestRobotsRateLimitFallsBackOnInvalidCrawlDelay(t *testing.T) {
	t.Parallel()

	robot := NewRobot(context.Background(), http.DefaultClient)
	parsed, err := url.Parse("https://example.com/path")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	robot.RuleHosts[parsed.Host] = &Rules{rules: map[string]*RuleSet{
		"Grawl": {crawlDelay: "not-a-number"},
	}}

	ticker := robot.RateLimit("Grawl", parsed)
	if ticker == nil {
		t.Fatalf("expected ticker fallback")
	}
	ticker.Stop()
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("read error")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
