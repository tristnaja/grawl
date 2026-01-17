package main

import (
	"context"
	"fmt"
	"log"
	"sync"
)

func main() {
	log.SetPrefix("grawl: ")
	log.SetFlags(0)

	startURL := "https://books.toscrape.com/"
	jobs := make(chan Job, 100)
	result := make(chan Result, 100)
	visMap := VisMap{
		Visited: make(map[string]struct{}),
	}
	numWorker := 10
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go worker(1, ctx, jobs, result, &wg)

	if visMap.ShouldCrawl(startURL) {
		job := Job{
			ID:  0,
			URL: startURL,
		}

		jobs <- job
		fmt.Println("Initial link sent")

	}

	for i := 1; i <= numWorker; i++ {
		wg.Add(1)
		go worker(i, ctx, jobs, result, &wg)
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	for links := range result {
		visMap.QueueTrack--
		for _, link := range links.Finding {
			if visMap.ShouldCrawl(link) {
				newJob := Job{
					ID:  links.JobID + 1,
					URL: link,
				}

				jobs <- newJob
			}
		}

		if visMap.QueueTrack == 0 {
			close(jobs)
		}
	}

	fmt.Println("Ready to print results:")

	for job := range jobs {
		fmt.Println(job.URL)
	}

	fmt.Println("finished crawling")
}
