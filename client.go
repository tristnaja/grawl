package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func fetch(url string) (*html.Node, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)

	if err != nil {
		return nil, fmt.Errorf("initiating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	doc, err := html.Parse(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}

	return doc, nil
}

func extractLink(ctx context.Context, rootLink string, node *html.Node) <-chan string {
	ch := make(chan string, 5)

	go func() {
		defer close(ch)
		var traversal func(*html.Node)
		seen := make(map[string]struct{})
		var finalLink string
		traversal = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						if _, exists := seen[attr.Val]; exists {
							continue
						} else {
							seen[attr.Val] = struct{}{}
							if strings.Contains(attr.Val, "https://") {
								finalLink = attr.Val
							} else {
								finalLink = rootLink + attr.Val
							}
							select {
							case <-ctx.Done():
								return
							default:
								ch <- finalLink
							}
						}
					}
				}
			}

			for child := n.FirstChild; child != nil; child = child.NextSibling {
				traversal(child)
			}
		}

		traversal(node)
	}()

	return ch
}
