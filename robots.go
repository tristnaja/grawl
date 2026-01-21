package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	Host   string
	Ticker *time.Ticker
}

type RobotsRule struct {
	UserAgent  string
	Disallow   []string
	Allow      []string
	CrawlDelay int
}

type Robots struct {
	mu             sync.Mutex
	DomainsLimiter map[string]*RateLimiter
	Rules          map[string]*RobotsRule
}

func (r *Robots) RateLimit(host string) *RateLimiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	var ticker *time.Ticker

	if rateLimit, exist := r.DomainsLimiter[host]; exist {
		return rateLimit
	}

	if _, exist := r.Rules[host]; exist {
		ticker = time.NewTicker(time.Duration(r.Rules[host].CrawlDelay) * time.Second)
	} else {

		ticker = time.NewTicker(5 * time.Second)
	}

	rateLimit := &RateLimiter{
		Host:   host,
		Ticker: ticker,
	}

	r.DomainsLimiter[host] = rateLimit

	return rateLimit
}

func (r *Robots) IsAllowed(agent, path string) (bool, error) {
	parsedURL, err := url.Parse(path)

	if err != nil {
		return false, fmt.Errorf("parsing url: %w", err)
	}

	rule, ok := r.Rules[agent]

	if !ok {
		rule, ok = r.Rules["*"]
		if !ok {
			return true, nil
		}
	}

	for _, disallow := range rule.Disallow {
		if disallow != "" && strings.HasPrefix(parsedURL.Path, disallow) {
			for _, allow := range rule.Allow {
				if strings.HasPrefix(parsedURL.Path, allow) && len(allow) >= len(disallow) {
					return true, nil
				}
			}

			return false, nil
		}
	}

	return true, nil
}

func ParseRobot(link string) (*Robots, error) {
	robotsURL := link + "robots.txt"
	resp, err := http.Get(robotsURL)

	if err != nil {
		return nil, fmt.Errorf("getting http request: %w", err)
	}

	defer resp.Body.Close()

	data := &Robots{
		DomainsLimiter: make(map[string]*RateLimiter),
		Rules:          make(map[string]*RobotsRule),
	}

	scanner := bufio.NewScanner(resp.Body)
	var currentAgent string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)

		if len(parts) < 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "user-agent":
			currentAgent = val
			if _, ok := data.Rules[currentAgent]; !ok {
				data.Rules[currentAgent] = &RobotsRule{UserAgent: val}
			}
		case "disallow":
			if currentAgent != "" {
				data.Rules[currentAgent].Disallow = append(data.Rules[currentAgent].Disallow, val)
			}
		case "allow":
			if currentAgent != "" {
				data.Rules[currentAgent].Allow = append(data.Rules[currentAgent].Allow, val)
			}
		case "crawl-delay":
			if currentAgent != "" {
				data.Rules[currentAgent].CrawlDelay, err = strconv.Atoi(val)
			}
		}
	}

	return data, nil
}
