package internal

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestOrchestrateCompletesWithDepthZero(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer srv.Close()

	jobs := make(chan Job, 8)
	results := make(chan Result, 8)
	errs := make(chan error, 8)

	var wg sync.WaitGroup

	done := make(chan struct{})
	go func() {
		if err := Orchestrate(srv.URL, jobs, results, errs, 1, 0, &wg); err != nil {
			t.Errorf("orchestrate returned error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("orchestrate did not complete in time")
	}
}
