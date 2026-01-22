package main

import (
	"context"
	"fmt"
	"log"
	"maps"
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

	robotsData, err := ParseRobot(startURL)

	if err != nil {
		log.Fatal(err)
	}

	jobs := make(chan Job, 100)
	result := make(chan Result, 100)
	numWorker := 10
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := WorkerInit(myAgent, ctx, jobs, result, robotsData, &wg)

	allowed, err := robotsData.IsAllowed(botName, startURL)

	if err != nil {
		log.Fatal(err)
	}

	for i := 1; i <= numWorker; i++ {
		wg.Add(1)
		go worker.Run(i)
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

			if _, exist := robotsData.host[parsedURL.Host]; !exist {
				fmt.Println("getting new rules")
				parsedURL.Path = "/"
				parsedURL.RawQuery = ""
				parsedURL.Fragment = ""

				fmt.Println(parsedURL.String())

				addedRules, err := ParseRobot(parsedURL.String())

				if err != nil {
					log.Fatal(err)
				}

				maps.Copy(robotsData.Rules, addedRules.Rules)
				robotsData.host[parsedURL.Host] = struct{}{}
			}

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
