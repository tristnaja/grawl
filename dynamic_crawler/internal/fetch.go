package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/html"
)

// fetchDocument downloads and parses one HTML page with retries.
func fetchDocument(ctx context.Context, client *http.Client, userAgent string, pageURL *url.URL, retries int, backoff time.Duration) (*html.Node, error) {
	if retries < 1 {
		retries = 1
	}

	var lastErr error

	for attempt := 1; attempt <= retries; attempt++ {
		attemptStart := time.Now()
		MetricInflightRequestsInc()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
		if err != nil {
			MetricInflightRequestsDec()
			MetricFetchDurationObserve(time.Since(attemptStart).Seconds())
			return nil, fmt.Errorf("building request: %w", err)
		}

		req.Header.Set("User-Agent", userAgent)
		resp, err := client.Do(req)
		MetricInflightRequestsDec()
		MetricFetchDurationObserve(time.Since(attemptStart).Seconds())
		if err != nil {
			lastErr = fmt.Errorf("performing request: %w", err)
		} else {
			if resp.StatusCode == http.StatusOK {
				doc, parseErr := html.Parse(resp.Body)
				resp.Body.Close()
				if parseErr != nil {
					lastErr = fmt.Errorf("parsing html: %w", parseErr)
				} else {
					return doc, nil
				}
			} else {
				resp.Body.Close()
				if resp.StatusCode >= 400 && resp.StatusCode < 500 {
					return nil, fmt.Errorf("non-retryable status code: %d", resp.StatusCode)
				}
				lastErr = fmt.Errorf("status code: %d", resp.StatusCode)
			}
		}

		if attempt == retries {
			break
		}
		MetricRetriesInc()

		select {
		case <-time.After(backoff * time.Duration(attempt)):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("fetch failed")
	}

	return nil, fmt.Errorf("fetching %s: %w", pageURL.String(), lastErr)
}

// collectLinks extracts unique absolute links from an HTML node.
func collectLinks(doc *html.Node, base *url.URL) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)

	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key != "href" {
					continue
				}

				rel, err := url.Parse(attr.Val)
				if err != nil {
					continue
				}

				resolved := base.ResolveReference(rel)
				if resolved.Scheme != "http" && resolved.Scheme != "https" {
					continue
				}

				link := resolved.String()
				if _, exists := seen[link]; exists {
					continue
				}

				seen[link] = struct{}{}
				result = append(result, link)
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(doc)
	return result
}
