package internal

import (
	"sync"
	"testing"
)

func TestSchedulerShouldCrawl(t *testing.T) {
	sch := &Scheduler{Visited: make(map[string]struct{})}

	if ok := sch.ShouldCrawl("https://example.com"); !ok {
		t.Fatalf("first visit should be allowed")
	}

	if ok := sch.ShouldCrawl("https://example.com"); ok {
		t.Fatalf("second visit should be rejected")
	}
}

func TestSchedulerShouldCrawlConcurrent(t *testing.T) {
	sch := &Scheduler{Visited: make(map[string]struct{})}

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)

	results := make(chan bool, workers)
	for i := range workers {
		_ = i
		go func() {
			defer wg.Done()
			results <- sch.ShouldCrawl("https://example.com/shared")
		}()
	}

	wg.Wait()
	close(results)

	allowedCount := 0
	for allowed := range results {
		if allowed {
			allowedCount++
		}
	}

	if allowedCount != 1 {
		t.Fatalf("expected exactly one allowed visit, got %d", allowedCount)
	}
}
