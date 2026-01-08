package main

import (
	"context"
	"fmt"
	"net/http"
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

func extractLink(node *html.Node) []string {
	var links []string
	var traversal func(*html.Node)

	traversal = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					links = append(links, attr.Val)
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			traversal(child)
		}
	}

	traversal(node)

	return links
}
