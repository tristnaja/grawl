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
	botName := "Grawl"
	myAgent := botName + "/1.0 (trstnalharrish@gmail.com)"

	var wg sync.WaitGroup
	scheduler := &Scheduler{
		Visited: make(map[string]struct{}),
	}

	startURL := "https://books.toscrape.com/"

	robotsData, err := ParseRobot(startURL)

	if err != nil {
		log.Fatal(err)
	}

	jobs := make(chan Job, 100)
	result := make(chan Result, 100)
	numWorker := 10
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	allowed, err := robotsData.IsAllowed(botName, startURL)

	if err != nil {
		log.Fatal(err)
	}

	if allowed {
		wg.Add(1)
		go worker(1, myAgent, ctx, jobs, result, robotsData, &wg)

		if scheduler.ShouldCrawl(startURL) {
			job := Job{
				ID:  0,
				URL: startURL,
			}

			jobs <- job
			fmt.Println("Initial link sent")

		}
	}

	for i := 1; i <= numWorker; i++ {
		wg.Add(1)
		go worker(i, myAgent, ctx, jobs, result, robotsData, &wg)
	}

	for links := range result {
		for _, link := range links.Finding {
			allowed, err := robotsData.IsAllowed(myAgent, link)

			if err != nil {
				log.Fatal(err)
			}

			if allowed {
				if scheduler.ShouldCrawl(link) {
					newJob := Job{
						ID:  links.JobID + 1,
						URL: link,
					}

					jobs <- newJob
				}
			}

		}
	}

	go func() {
		wg.Wait()
		close(jobs)
		close(result)
	}()

	fmt.Println("Ready to print results:")

	for job := range jobs {
		fmt.Println(job.URL)
	}

	fmt.Println("finished crawling")
}
