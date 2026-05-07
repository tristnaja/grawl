package internal

import (
	"sync"
	"testing"
)

func TestSchedulerTryVisit(t *testing.T) {
	s := newScheduler()

	if !s.tryVisit("https://example.com") {
		t.Fatalf("expected first visit allowed")
	}

	if s.tryVisit("https://example.com") {
		t.Fatalf("expected duplicate visit rejected")
	}

	if got := s.count(); got != 1 {
		t.Fatalf("expected count=1, got %d", got)
	}
}

func TestSchedulerTryVisitConcurrent(t *testing.T) {
	s := newScheduler()

	const n = 30
	var wg sync.WaitGroup
	wg.Add(n)

	results := make(chan bool, n)
	for range n {
		go func() {
			defer wg.Done()
			results <- s.tryVisit("https://example.com/same")
		}()
	}

	wg.Wait()
	close(results)

	allowed := 0
	for ok := range results {
		if ok {
			allowed++
		}
	}

	if allowed != 1 {
		t.Fatalf("expected 1 successful visit, got %d", allowed)
	}
}
