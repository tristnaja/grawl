package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

//	type RateLimiter struct {
//		Host   string
//		Ticker *time.Ticker
//	}
type RuleSet struct {
	userAgent  string
	disallow   []string
	allow      []string
	crawlDelay string
}

type Rules struct {
	rules map[string]*RuleSet
}

type Robots struct {
	mu             sync.Mutex
	ctx            context.Context
	client         *http.Client
	DomainsLimiter map[string]*time.Ticker
	RuleHosts      map[string]*Rules
}

func NewRobot(ctx context.Context, client *http.Client) *Robots {
	return &Robots{
		ctx:            ctx,
		client:         client,
		DomainsLimiter: make(map[string]*time.Ticker),
		RuleHosts:      make(map[string]*Rules),
	}
}

func (r *Robots) FetchRules(link string) error {
	parsedURL, err := url.Parse(link)

	if err != nil {
		return fmt.Errorf("parsing url: %w", err)
	}

	parsedURL.Path = "/robots.txt"
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	host := parsedURL.Host

	r.mu.Lock()
	if _, exist := r.RuleHosts[host]; exist {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, parsedURL.String(), nil)

	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := r.client.Do(req)

	if err != nil {
		return fmt.Errorf("getting response from an http request: %w", err)
	}

	defer resp.Body.Close()
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exist := r.RuleHosts[host]; exist {
		return nil
	}

	rule := &RuleSet{
		userAgent:  "",
		disallow:   make([]string, 0),
		allow:      make([]string, 0),
		crawlDelay: "",
	}

	cache := &Rules{
		rules: make(map[string]*RuleSet),
	}

	var currActiveAgent []string
	scanner := bufio.NewScanner(resp.Body)

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
			if _, exist := cache.rules[val]; !exist {
				rule.userAgent = val
				cache.rules[val] = rule
			}

			currActiveAgent = append(currActiveAgent, val)
		case "allow", "disallow", "crawl-delay":
			if len(currActiveAgent) == 0 {
				continue
			}

			for _, agent := range currActiveAgent {
				switch key {
				case "allow":
					cache.rules[agent].allow = append(cache.rules[agent].allow, val)
				case "disallow":
					cache.rules[agent].disallow = append(cache.rules[agent].disallow, val)
				case "crawl-delay":
					cache.rules[agent].crawlDelay = val
				}
			}
		case "":
			currActiveAgent = []string{}
		}
	}

	r.RuleHosts[host] = cache
	return nil
}

func (r *Robots) RateLimit(agent string, link *url.URL) *time.Ticker {
	r.mu.Lock()
	defer r.mu.Unlock()

	bufRand := rand.Float64()

	var ticker *time.Ticker
	var tickerAmount float64
	var err error

	if rateLimit, exist := r.DomainsLimiter[link.Host]; exist {
		return rateLimit
	}

	if ticker, exist := r.DomainsLimiter[link.Host]; exist {
		return ticker
	}

	host := r.RuleHosts[link.Host]

	if host != nil {
		rules, exist := host.rules[agent]

		if !exist {
			rules, _ = host.rules["*"]
		}

		if rules != nil && rules.crawlDelay != "" {
			tickerAmount, err = strconv.ParseFloat(strings.TrimSpace(rules.crawlDelay), 64)
		}
	}

	if err != nil || tickerAmount <= 0 {
		tickerAmount = 2 + bufRand
	}

	if tickerAmount > 30 {
		tickerAmount = 30
	}

	ticker = time.NewTicker(time.Duration(tickerAmount) * time.Second)
	r.DomainsLimiter[link.Host] = ticker

	return ticker
}

func (r *Robots) IsAllowed(agent, path string) (bool, error) {
	parsedURL, err := url.Parse(path)

	if err != nil {
		return false, fmt.Errorf("parsing url: %w", err)
	}

	r.mu.Lock()
	base, exist := r.RuleHosts[parsedURL.Host]
	r.mu.Unlock()

	if !exist {
		fmt.Printf("\033[33m|getting new rules:\n|%v\n\033[0m", parsedURL.Host)

		if err := r.FetchRules(parsedURL.String()); err != nil {
			return true, fmt.Errorf("robots cannot be parsed: %w", err)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	base, exist = r.RuleHosts[parsedURL.Host]

	if !exist {
		return true, nil
	}

	rule, exist := base.rules[agent]

	if !exist {
		rule, exist = base.rules["*"]
		if !exist {
			return true, nil
		}
	}

	for _, disallow := range rule.disallow {
		if disallow != "" && strings.HasPrefix(parsedURL.Path, disallow) {
			for _, allow := range rule.allow {
				if strings.HasPrefix(parsedURL.Path, allow) && len(allow) >= len(disallow) {
					return true, nil
				}
			}

			return false, nil
		}
	}

	return true, nil
}

// func ParseRobot(link string) (*Robots, error) {
// 	robotsURL, err := url.Parse(link)
//
// 	if err != nil {
// 		return nil, fmt.Errorf("parsing base url <robots.txt>: %w", err)
// 	}
//
// 	robotsURL.Path = "robots.txt"
//
// 	resp, err := http.Get(robotsURL.String())
//
// 	if err != nil {
// 		return nil, fmt.Errorf("getting http request: %w", err)
// 	}
//
// 	defer resp.Body.Close()
//
// 	cache := &Rules{
// 		rules: make(map[string]*RuleSet),
// 	}
//
// 	data := &Robots{
// 		DomainsLimiter: make(map[string]*RateLimiter),
// 		Host:           make(map[string]*Rules),
// 	}
//
// 	scanner := bufio.NewScanner(resp.Body)
// 	var currentAgent string
//
// 	for scanner.Scan() {
// 		line := strings.TrimSpace(scanner.Text())
//
// 		if line == "" || strings.HasPrefix(line, "#") {
// 			continue
// 		}
//
// 		parts := strings.SplitN(line, ":", 2)
//
// 		if len(parts) < 2 {
// 			continue
// 		}
//
// 		key := strings.ToLower(strings.TrimSpace(parts[0]))
// 		val := strings.TrimSpace(parts[1])
//
// 		switch key {
// 		case "user-agent":
// 			currentAgent = val
// 			if _, ok := cache.rules[currentAgent]; !ok {
// 				cache.rules[currentAgent] = &RuleSet{userAgent: val}
// 			}
// 		case "disallow":
// 			if currentAgent != "" {
// 				cache.rules[currentAgent].disallow = append(cache.rules[currentAgent].disallow, val)
// 			}
// 		case "allow":
// 			if currentAgent != "" {
// 				cache.rules[currentAgent].allow = append(cache.rules[currentAgent].allow, val)
// 			}
// 		case "crawl-delay":
// 			if currentAgent != "" {
// 				cache.rules[currentAgent].crawlDelay, err = strconv.Atoi(val)
// 			}
// 		}
// 	}
//
// 	data.Host[robotsURL.Host] = cache
//
// 	return data, nil
// }
