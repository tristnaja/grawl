package internal

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type robotsRuleSet struct {
	disallow []string
	allow    []string
}

type robotsRules struct {
	agents map[string]*robotsRuleSet
}

type robotsStore struct {
	mu        sync.Mutex
	ctx       context.Context
	client    *http.Client
	rules     map[string]*robotsRules
	limit     map[string]*time.Ticker
	baseDelay time.Duration
}

func newRobotsStore(ctx context.Context, client *http.Client, baseDelay time.Duration) *robotsStore {
	if baseDelay <= 0 {
		baseDelay = 2 * time.Second
	}

	return &robotsStore{
		ctx:       ctx,
		client:    client,
		rules:     make(map[string]*robotsRules),
		limit:     make(map[string]*time.Ticker),
		baseDelay: baseDelay,
	}
}

func (r *robotsStore) allow(agent, rawURL string) (bool, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false, fmt.Errorf("parsing url: %w", err)
	}

	if err := r.ensure(parsed); err != nil {
		return true, fmt.Errorf("fetching robots: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	hostRules, ok := r.rules[parsed.Host]
	if !ok {
		return true, nil
	}

	set, ok := hostRules.agents[agent]
	if !ok {
		set = hostRules.agents["*"]
		if set == nil {
			return true, nil
		}
	}

	path := parsed.Path
	for _, disallow := range set.disallow {
		if disallow != "" && strings.HasPrefix(path, disallow) {
			for _, allow := range set.allow {
				if strings.HasPrefix(path, allow) && len(allow) >= len(disallow) {
					return true, nil
				}
			}
			return false, nil
		}
	}

	return true, nil
}

func (r *robotsStore) wait(link *url.URL) {
	r.mu.Lock()
	ticker, ok := r.limit[link.Host]
	if !ok {
		ticker = time.NewTicker(r.baseDelay)
		r.limit[link.Host] = ticker
	}
	r.mu.Unlock()

	<-ticker.C
}

func (r *robotsStore) ensure(parsed *url.URL) error {
	host := parsed.Host

	r.mu.Lock()
	if _, exists := r.rules[host]; exists {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	robotsURL := &url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   "/robots.txt",
	}

	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, robotsURL.String(), nil)
	if err != nil {
		return fmt.Errorf("creating robots request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing robots request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		r.mu.Lock()
		r.rules[host] = &robotsRules{agents: map[string]*robotsRuleSet{}}
		r.mu.Unlock()
		return nil
	}

	parsedRules := &robotsRules{agents: make(map[string]*robotsRuleSet)}
	activeAgents := make([]string, 0)
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch key {
		case "user-agent":
			activeAgents = []string{value}
			if _, ok := parsedRules.agents[value]; !ok {
				parsedRules.agents[value] = &robotsRuleSet{}
			}
		case "allow":
			for _, agent := range activeAgents {
				parsedRules.agents[agent].allow = append(parsedRules.agents[agent].allow, value)
			}
		case "disallow":
			for _, agent := range activeAgents {
				parsedRules.agents[agent].disallow = append(parsedRules.agents[agent].disallow, value)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading robots response: %w", err)
	}

	r.mu.Lock()
	r.rules[host] = parsedRules
	r.mu.Unlock()

	return nil
}
