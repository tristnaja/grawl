package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
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

	startURL := "https://example.com/"
	jobs := make(chan Job, 100)
	result := make(chan Result, 100)
	numWorker := 10
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient(myAgent, ctx)
	robot := NewRobot(client.Ctx, client.Client)
	worker := NewWorker(*client, jobs, result, robot, &wg)

	allowed, err := robot.IsAllowed(botName, startURL)

	if err != nil {
		log.Fatal(err)
	}

	for i := 1; i <= numWorker; i++ {
		wg.Add(1)
		go worker.Run(i, botName)
	}

	if allowed {
		if scheduler.ShouldCrawl(startURL) {
			job := Job{
				ID:  0,
				URL: startURL,
			}

			jobs <- job
			fmt.Println("Initial link sent")
		}
	}

	for links := range result {
		scheduler.QueueTrack--
		for _, link := range links.Finding {
			parsedURL, err := url.Parse(link)

			if err != nil {
				log.Fatal(err)
			}

			if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				continue
			}

			allowed, err := robot.IsAllowed(botName, link)

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

		if scheduler.QueueTrack == 0 {
			close(jobs)
		}
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	fmt.Println("Ready to print results:")

	for job := range jobs {
		fmt.Println(job.URL)
	}

	fmt.Println("finished crawling")
}
