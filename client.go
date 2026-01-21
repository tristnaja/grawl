package main

import (
	"context"
	"fmt"
	"net/url"
	"sync"
)

type Job struct {
	ID  int
	URL string
}

type Result struct {
	JobID    int
	StartURL *url.URL
	Finding  []string
}

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
	return true
}

func worker(id int, myAgent string, ctx context.Context, jobs <-chan Job, result chan<- Result, robotsData *Robots, wg *sync.WaitGroup) error {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			job, ok := <-jobs

			if !ok {
				return fmt.Errorf("there is no initial link that can be processed")
			}

			parsedURL, err := url.Parse(job.URL)

			if err != nil {
				return fmt.Errorf("parsing url: %w", err)
			}

			ticker := robotsData.RateLimit(parsedURL.Host)

			<-ticker.Ticker.C

			doc, err := Fetch(ctx, myAgent, parsedURL)

			if err != nil {
				return err
			}

			fmt.Printf("Worker %d is Processing %v\n", id, parsedURL.String())
			res := Result{
				JobID:    job.ID,
				StartURL: parsedURL,
				Finding:  make([]string, 0),
			}

			Traverse(doc, res, result)
		}
	}
}
