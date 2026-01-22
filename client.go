package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Worker struct {
	AgentName string
	client    *http.Client
	ctx       context.Context
	Jobs      <-chan Job
	Result    chan<- Result
	robots    *Robots
	wg        *sync.WaitGroup
}

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
	mu         sync.Mutex
	Visited    map[string]struct{}
	QueueTrack int
}

func (sch *Scheduler) ShouldCrawl(link string) bool {
	sch.mu.Lock()
	defer sch.mu.Unlock()

	if _, exist := sch.Visited[link]; exist {
		return false
	}

	sch.Visited[link] = struct{}{}
	sch.QueueTrack++
	return true
}

func WorkerInit(myAgent string, ctx context.Context, jobs <-chan Job, result chan<- Result, robotsData *Robots, wg *sync.WaitGroup) *Worker {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          100,
		MaxConnsPerHost:       10,
		IdleConnTimeout:       90 * time.Second,
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	return &Worker{
		AgentName: myAgent,
		client:    client,
		ctx:       ctx,
		Jobs:      jobs,
		Result:    result,
		robots:    robotsData,
		wg:        wg,
	}
}

func (worker *Worker) Run(id int) error {
	defer worker.wg.Done()

	for {
		select {
		case <-worker.ctx.Done():
			return worker.ctx.Err()
		case job, ok := <-worker.Jobs:
			if !ok {
				return fmt.Errorf("there is no initial link that can be processed")
			}

			parsedURL, err := url.Parse(job.URL)

			if err != nil {
				return fmt.Errorf("parsing url: %w", err)
			}

			ticker := worker.robots.RateLimit(parsedURL.Host)

			<-ticker.Ticker.C

			doc, err := Fetch(worker.ctx, worker.client, worker.AgentName, parsedURL)

			if err != nil {
				return err
			}

			fmt.Printf("Worker %d is Processing %v\n", id, parsedURL.String())
			res := Result{
				JobID:    job.ID,
				StartURL: parsedURL,
				Finding:  make([]string, 0),
			}

			Traverse(doc, res, worker.Result)
		}
	}
}
