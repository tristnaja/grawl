package internal

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type crawlJob struct {
	URL   string
	Depth int
}

// Crawler executes a dynamic goroutine-per-task crawl.
type Crawler struct {
	config Config
}

func NewCrawler(config Config) *Crawler {
	if config.MaxDepth <= 0 {
		config.MaxDepth = 2
	}

	if config.HTTPTimeout <= 0 {
		config.HTTPTimeout = 30 * time.Second
	}

	if config.RetryCount <= 0 {
		config.RetryCount = 3
	}

	if config.RetryBackoff <= 0 {
		config.RetryBackoff = 2 * time.Second
	}

	if config.UserAgent == "" {
		config.UserAgent = "Grawl-Dynamic/1.0"
	}

	if config.BaseDelay <= 0 {
		config.BaseDelay = 2 * time.Second
	}

	return &Crawler{config: config}
}

func (c *Crawler) Crawl(ctx context.Context, startURL string) (Result, error) {
	start, err := normalizeStartURL(startURL)
	if err != nil {
		return Result{}, err
	}

	httpClient := newHTTPClient(c.config.HTTPTimeout)
	robots := newRobotsStore(ctx, httpClient, c.config.BaseDelay)
	scheduler := newScheduler()

	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	var visited atomic.Int64
	var allowed atomic.Int64
	var discovered atomic.Int64

	sendErr := func(err error) {
		select {
		case errChan <- err:
		default:
		}
	}

	var crawl func(job crawlJob)
	crawl = func(job crawlJob) {
		defer wg.Done()
		defer MetricActiveGoroutinesDec()

		if ctx.Err() != nil {
			return
		}

		if job.Depth > c.config.MaxDepth {
			return
		}

		if job.URL == "" {
			return
		}

		if !scheduler.tryVisit(job.URL) {
			return
		}

		visited.Add(1)

		isAllowed, err := robots.allow(c.config.UserAgent, job.URL)
		if err != nil {
			sendErr(fmt.Errorf("robots check failed for %s: %w", job.URL, err))
			return
		}

		if !isAllowed {
			MetricRobotsDeniedInc()
			return
		}

		MetricURLsAllowedInc()
		allowed.Add(1)

		parsed, err := url.Parse(job.URL)
		if err != nil {
			sendErr(fmt.Errorf("parsing url %s: %w", job.URL, err))
			return
		}

		robots.wait(parsed)

		doc, err := fetchDocument(ctx, httpClient, c.config.UserAgent, parsed, c.config.RetryCount, c.config.RetryBackoff)
		if err != nil {
			MetricFetchErrorsInc()
			sendErr(err)
			return
		}

		nextLinks := collectLinks(doc, parsed)
		MetricURLsDiscoveredAdd(len(nextLinks))
		discovered.Add(int64(len(nextLinks)))

		for _, link := range nextLinks {
			wg.Add(1)
			MetricActiveGoroutinesInc()
			go crawl(crawlJob{URL: link, Depth: job.Depth + 1})
		}
	}

	wg.Add(1)
	MetricActiveGoroutinesInc()
	go crawl(crawlJob{URL: start.String(), Depth: 0})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errChan:
		return Result{}, err
	case <-done:
		return Result{
			Visited:    int(visited.Load()),
			Allowed:    int(allowed.Load()),
			Discovered: int(discovered.Load()),
		}, nil
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

func normalizeStartURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing start URL: %w", err)
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", parsed.Scheme)
	}

	if parsed.Host == "" {
		return nil, fmt.Errorf("start URL must include host")
	}

	return parsed, nil
}
