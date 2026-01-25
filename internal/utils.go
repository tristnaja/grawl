package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/html"
)

func FetchPage(ctx context.Context, client *http.Client, myAgent string, parsedURL *url.URL) (*html.Node, error) {
	maxRetries := 3
	var err error

	for i := range maxRetries {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)

		if err != nil {
			return nil, fmt.Errorf("formulating request: %w", err)
		}

		req.Header.Set("User-Agent", myAgent)

		resp, doErr := client.Do(req)
		err = doErr

		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				doc, parseErr := html.Parse(resp.Body)
				err = parseErr

				if err != nil {
					return nil, fmt.Errorf("parsing http response: %w", err)
				}

				return doc, nil
			}
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return nil, fmt.Errorf("client error: %d", resp.StatusCode)
			}

		}

		if i < maxRetries {
			waitTime := time.Duration(i+1) * 2 * time.Second

			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, err)
}

func TraverseNode(node *html.Node, res *Result) {
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
}
