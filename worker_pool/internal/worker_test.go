package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestWorkerRunProcessesJob(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body><a href='/next'>next</a></body></html>"))
	}))
	defer srv.Close()

	client := NewClient()
	client.Client = srv.Client()

	robot := NewRobot(client.Context.ctx, client.Client)
	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	robot.DomainsLimiter[parsed.Host] = ticker

	jobs := make(chan Job, 1)
	results := make(chan Result, 1)
	errs := make(chan error, 2)

	var wg sync.WaitGroup
	wg.Add(1)
	worker := NewWorker(*client, jobs, results, errs, robot, &wg)
	go worker.Run(1)

	jobs <- Job{ID: 7, URL: srv.URL, CurrentDepth: 0}

	select {
	case res := <-results:
		if res.JobID != 7 {
			t.Fatalf("unexpected job id: %d", res.JobID)
		}
		if res.CurrentDepth != 0 {
			t.Fatalf("unexpected current depth: %d", res.CurrentDepth)
		}
		if len(res.Finding) != 1 {
			t.Fatalf("expected one discovered link, got %d", len(res.Finding))
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for worker result")
	}

	close(jobs)
	waitGroupOrTimeout(t, &wg, 2*time.Second)
}

func TestWorkerRunOnClosedJobsEmitsError(t *testing.T) {
	t.Parallel()

	client := NewClient()
	robot := NewRobot(context.Background(), client.Client)

	jobs := make(chan Job)
	results := make(chan Result, 1)
	errs := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	worker := NewWorker(*client, jobs, results, errs, robot, &wg)

	close(jobs)
	go worker.Run(99)

	select {
	case err := <-errs:
		if err == nil {
			t.Fatalf("expected non-nil error")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for worker error")
	}

	waitGroupOrTimeout(t, &wg, 2*time.Second)
}

func TestWorkerRunInvalidJobURLEmitsError(t *testing.T) {
	t.Parallel()

	client := NewClient()
	robot := NewRobot(context.Background(), client.Client)

	jobs := make(chan Job, 1)
	results := make(chan Result, 1)
	errs := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	worker := NewWorker(*client, jobs, results, errs, robot, &wg)
	go worker.Run(2)

	jobs <- Job{ID: 1, URL: "://bad", CurrentDepth: 0}

	select {
	case err := <-errs:
		if err == nil {
			t.Fatalf("expected parse error")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for parse error")
	}

	waitGroupOrTimeout(t, &wg, 2*time.Second)
}

func TestWorkerRunFetchFailureEmitsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient()
	client.Client = srv.Client()

	robot := NewRobot(client.Context.ctx, client.Client)
	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	robot.DomainsLimiter[parsed.Host] = ticker

	jobs := make(chan Job, 1)
	results := make(chan Result, 1)
	errs := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	worker := NewWorker(*client, jobs, results, errs, robot, &wg)
	go worker.Run(3)

	jobs <- Job{ID: 9, URL: srv.URL, CurrentDepth: 0}

	select {
	case err := <-errs:
		if err == nil {
			t.Fatalf("expected fetch error")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for fetch error")
	}

	waitGroupOrTimeout(t, &wg, 2*time.Second)
}

func TestWorkerRunCancelledContextEmitsError(t *testing.T) {
	t.Parallel()

	client := NewClient()
	robot := NewRobot(client.Context.ctx, client.Client)

	jobs := make(chan Job)
	results := make(chan Result, 1)
	errs := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	worker := NewWorker(*client, jobs, results, errs, robot, &wg)

	client.Context.cancel()
	go worker.Run(4)

	select {
	case err := <-errs:
		if err == nil {
			t.Fatalf("expected context cancellation error")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for cancellation error")
	}

	waitGroupOrTimeout(t, &wg, 2*time.Second)
}

func waitGroupOrTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("waitgroup timeout after %v", timeout)
	}
}
