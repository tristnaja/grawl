package internal

import "net/url"

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
