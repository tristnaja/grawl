package internal

import "time"

// Config defines crawl behavior for the semaphore crawler.
type Config struct {
	MaxDepth     int
	UserAgent    string
	HTTPTimeout  time.Duration
	RetryCount   int
	RetryBackoff time.Duration
	BaseDelay    time.Duration
	Quiet        bool
	MetricsAddr  string
	PProfAddr    string
}

// Result summarizes crawl execution metrics.
type Result struct {
	Visited    int
	Allowed    int
	Discovered int
}
