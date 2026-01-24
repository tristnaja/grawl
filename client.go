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

type Client struct {
	AgentName string
	Client    *http.Client
	Ctx       context.Context
}

type Worker struct {
	Client Client
	Jobs   <-chan Job
	Result chan<- Result
	robots *Robots
	wg     *sync.WaitGroup
}

type Job struct {
	ID           int
	URL          string
	CurrentDepth int
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

func NewClient(agentName string, ctx context.Context) *Client {
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

	return &Client{
		AgentName: agentName,
		Client:    client,
		Ctx:       ctx,
	}
}

func NewWorker(client Client, jobs <-chan Job, result chan<- Result, robotsData *Robots, wg *sync.WaitGroup) *Worker {
	return &Worker{
		Client: client,
		Jobs:   jobs,
		Result: result,
		robots: robotsData,
		wg:     wg,
	}
}

func (worker *Worker) Run(id int, botName string) error {
	defer worker.wg.Done()

	for {
		select {
		case <-worker.Client.Ctx.Done():
			return worker.Client.Ctx.Err()
		case job, ok := <-worker.Jobs:
			if !ok {
				return fmt.Errorf("there is no initial link that can be processed")
			}

			parsedURL, err := url.Parse(job.URL)

			if err != nil {
				return fmt.Errorf("parsing url: %w", err)
			}

			ticker := worker.robots.RateLimit(botName, parsedURL)

			<-ticker.C

			doc, err := Fetch(worker.Client.Ctx, worker.Client.Client, worker.Client.AgentName, parsedURL)

			if err != nil {
				return err
			}

			fmt.Println("----------------------------------------")
			fmt.Printf("Worker %d\nProcessing %v\nDepth: %d\n", id, parsedURL.String(), job.CurrentDepth)
			fmt.Println("----------------------------------------")
			res := Result{
				JobID:    job.ID,
				StartURL: parsedURL,
				Finding:  make([]string, 0),
			}

			Traverse(doc, &res)

			worker.Result <- res
		}
	}
}
