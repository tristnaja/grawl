package internal

import "sync"

type Scheduler struct {
	mu      sync.Mutex
	Visited map[string]struct{}
}

func (sch *Scheduler) ShouldCrawl(link string) bool {
	sch.mu.Lock()
	defer sch.mu.Unlock()

	if _, exist := sch.Visited[link]; exist {
		return false
	}

	sch.Visited[link] = struct{}{}
	MetricURLsVisitedInc()
	return true
}
