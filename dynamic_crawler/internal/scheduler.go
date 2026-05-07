package internal

import "sync"

type scheduler struct {
	mu      sync.Mutex
	visited map[string]struct{}
}

func newScheduler() *scheduler {
	return &scheduler{visited: make(map[string]struct{})}
}

func (s *scheduler) tryVisit(link string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.visited[link]; exists {
		return false
	}

	s.visited[link] = struct{}{}
	MetricURLsVisitedInc()
	return true
}

func (s *scheduler) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.visited)
}
