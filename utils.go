package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/html"
)

func Fetch(ctx context.Context, myAgent string, parsedURL *url.URL) (*html.Node, error) {
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

	maxRetries := 3
	var err error

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)

		if err != nil {
			return nil, fmt.Errorf("formulating request: %w", err)
		}

		req.Header.Set("User-Agent", myAgent)

		resp, err := client.Do(req)

		if err != nil {
			return nil, fmt.Errorf("requesting response: %w", err)
		}

		defer resp.Body.Close()

		if err == nil && resp.StatusCode == http.StatusOK {
			doc, err := html.Parse(resp.Body)

			if err != nil {
				return nil, fmt.Errorf("parsing http response: %w", err)
			}

			return doc, nil
		}

		if resp != nil && resp.StatusCode >= 400 && resp.StatusCode < 500 {
			resp.Body.Close()
			return nil, fmt.Errorf("client error: %d", resp.StatusCode)
		}

		if i < maxRetries {
			waitTime := time.Duration(i+1) * 2 * time.Second
			time.Sleep(waitTime)
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", err)
}

func Traverse(node *html.Node, res Result, result chan<- Result) {
	seen := make(map[string]struct{})
	var traversal func(node *html.Node)

	traversal = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" {
					if _, exist := seen[attr.Val]; !exist {
						seen[attr.Val] = struct{}{}
						relLink, err := url.Parse(attr.Val)

						if err != nil {
							continue
						}

						resolved := res.StartURL.ResolveReference(relLink)
						res.Finding = append(res.Finding, resolved.String())
					}
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			traversal(child)
		}
	}

	traversal(node)
	result <- res
}
