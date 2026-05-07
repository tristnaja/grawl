package internal

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestNewManagerInitializesScheduler(t *testing.T) {
	jobs := make(chan Job)
	results := make(chan Result)
	errs := make(chan error)

	client := NewClient()
	robots := NewRobot(context.Background(), client.Client)

	manager := NewManager(*client, jobs, results, errs, robots)
	if manager == nil {
		t.Fatalf("expected manager instance")
	}

	if manager.Scheduler == nil || manager.Scheduler.Visited == nil {
		t.Fatalf("expected initialized scheduler")
	}
}

func TestManagerRunCompletesAndSignalsDone(t *testing.T) {
	jobs := make(chan Job, 4)
	results := make(chan Result, 4)
	errs := make(chan error, 4)

	client := NewClient()
	robots := NewRobot(context.Background(), http.DefaultClient)
	seedAllowAllRule(t, robots, "example.com")

	manager := NewManager(*client, jobs, results, errs, robots)

	go manager.Run("https://example.com", 2)

	first := <-jobs
	if first.URL != "https://example.com" {
		t.Fatalf("unexpected first job URL: %s", first.URL)
	}

	startURL, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatalf("parse start URL failed: %v", err)
	}

	results <- Result{JobID: first.ID, CurrentDepth: 0, StartURL: startURL, Finding: []string{"https://example.com/next"}}

	second := <-jobs
	if second.URL != "https://example.com/next" {
		t.Fatalf("unexpected second job URL: %s", second.URL)
	}

	results <- Result{JobID: second.ID, CurrentDepth: 1, StartURL: startURL, Finding: nil}

	select {
	case done := <-errs:
		if done != nil {
			t.Fatalf("expected nil completion sentinel, got: %v", done)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for manager completion signal")
	}
}

func TestManagerRunReportsInvalidResultURL(t *testing.T) {
	jobs := make(chan Job, 4)
	results := make(chan Result, 4)
	errs := make(chan error, 4)

	client := NewClient()
	robots := NewRobot(context.Background(), http.DefaultClient)
	seedAllowAllRule(t, robots, "example.com")

	manager := NewManager(*client, jobs, results, errs, robots)

	go manager.Run("https://example.com", 1)

	first := <-jobs
	startURL, err := url.Parse(first.URL)
	if err != nil {
		t.Fatalf("parse first URL failed: %v", err)
	}

	results <- Result{JobID: first.ID, CurrentDepth: 0, StartURL: startURL, Finding: []string{"://bad"}}

	select {
	case reported := <-errs:
		if reported == nil {
			t.Fatalf("expected parse error from invalid discovered URL")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for manager error")
	}
}

func TestManagerRunSkipsNonHTTPDiscoveredLinks(t *testing.T) {
	jobs := make(chan Job, 4)
	results := make(chan Result, 4)
	errs := make(chan error, 4)

	client := NewClient()
	robots := NewRobot(context.Background(), http.DefaultClient)
	seedAllowAllRule(t, robots, "example.com")

	manager := NewManager(*client, jobs, results, errs, robots)
	go manager.Run("https://example.com", 2)

	first := <-jobs
	startURL, err := url.Parse(first.URL)
	if err != nil {
		t.Fatalf("parse first URL failed: %v", err)
	}

	results <- Result{JobID: first.ID, CurrentDepth: 0, StartURL: startURL, Finding: []string{"mailto:test@example.com"}}

	select {
	case done := <-errs:
		if done != nil {
			t.Fatalf("expected nil completion sentinel, got: %v", done)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for completion")
	}
}

func seedAllowAllRule(t *testing.T, robot *Robots, host string) {
	t.Helper()

	robot.RuleHosts[host] = &Rules{rules: map[string]*RuleSet{
		"Grawl": {
			allow:    []string{"/"},
			disallow: []string{},
		},
	}}
}
