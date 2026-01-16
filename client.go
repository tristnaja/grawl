package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"golang.org/x/net/html"
)

type Job struct {
	ID  int
	URL string
}

type Result struct {
	JobID    int
	StartURL string
	Finding  []string
}

func worker(id int, ctx context.Context, jobs <-chan Job, result chan<- Result, wg *sync.WaitGroup) error {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			job, ok := <-jobs

			if !ok {
				return fmt.Errorf("something went wrong")
			}

			parsedURL, err := url.Parse(job.URL)

			if err != nil {
				return fmt.Errorf("parsing url: %w", err)
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)

			if err != nil {
				return fmt.Errorf("formulating request: %w", err)
			}

			resp, err := http.DefaultClient.Do(req)

			if err != nil {
				return fmt.Errorf("requesting response: %w", err)
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return fmt.Errorf("status code is not OK: %d", resp.StatusCode)
			}

			doc, err := html.Parse(resp.Body)

			if err != nil {
				return fmt.Errorf("parsing http response: %w", err)
			}

			fmt.Printf("Worker %d is Processing %v\n", id, parsedURL.String())
			wg.Go(func() {
				res := Result{
					JobID:    job.ID,
					StartURL: parsedURL.String(),
					Finding:  make([]string, 0),
				}

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

									resolved := parsedURL.ResolveReference(relLink)
									res.Finding = append(res.Finding, resolved.String())
								}
							}
						}
					}

					for child := node.FirstChild; child != nil; child = child.NextSibling {
						traversal(child)
					}
				}

				traversal(doc)
				result <- res
			})
		}
	}
}
