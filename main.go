package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
)

func main() {
	// TODO: Make a TUI
	log.SetPrefix("grawl: ")
	log.SetFlags(0)
	botName := "Grawl"
	myAgent := botName + "/1.0 (trstnalharrish@gmail.com)"

	var wg sync.WaitGroup
	scheduler := &Scheduler{
		Visited: make(map[string]struct{}),
	}

	startURL := "https://wikipedia.com/"
	jobs := make(chan Job, 100)
	result := make(chan Result, 100)
	numWorker := 10
	maxCrawlDepth := 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient(myAgent, ctx)
	robot := NewRobot(client.Ctx, client.Client)
	worker := NewWorker(*client, jobs, result, robot, &wg)

	for i := 1; i <= numWorker; i++ {
		wg.Add(1)
		go worker.Run(i, botName)
	}

	go func() {
		var workerQueue []Job
		activeWorker := 0
		allowed, err := robot.IsAllowed(botName, startURL)

		if err != nil {
			log.Println(err)
		}

		if allowed {
			if scheduler.ShouldCrawl(startURL) {
				initialJob := Job{
					ID:           0,
					URL:          startURL,
					CurrentDepth: 0,
				}
				workerQueue = append(workerQueue, initialJob)
				fmt.Println("Initial link sent")
			}
		}

		for {
			var activeJobs chan Job
			var nextJob Job

			if len(workerQueue) > 0 {
				activeJobs = jobs
				nextJob = workerQueue[0]
			}

			if nextJob.CurrentDepth != maxCrawlDepth {
				select {
				case activeJobs <- nextJob:
					workerQueue = workerQueue[1:]
					activeWorker++
				case links := <-result:
					activeWorker--
					for _, link := range links.Finding {
						parsedURL, err := url.Parse(link)

						if err != nil {
							log.Println(err)
						}

						if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
							continue
						}

						allowed, err := robot.IsAllowed(botName, link)

						if err != nil {
							log.Println(err)
						}

						if allowed {
							if scheduler.ShouldCrawl(link) {
								newJob := Job{
									ID:           nextJob.ID + 1,
									URL:          link,
									CurrentDepth: nextJob.CurrentDepth + 1,
								}
								workerQueue = append(workerQueue, newJob)
							}
						}

					}

				}
			} else {
				workerQueue = []Job{}
				activeWorker = 0
			}

			if len(workerQueue) == 0 && activeWorker == 0 {
				close(jobs)
				return
			}
		}
	}()

	wg.Wait()
	close(result)
	fmt.Println("finished crawling")
}
