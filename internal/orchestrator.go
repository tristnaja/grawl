package internal

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
)

func Orchestrate(startURL string, jobs chan Job, result chan Result, errChan chan error, numWorker int, maxCrawlDepth int, wg *sync.WaitGroup) {
	log.SetPrefix("\033[33mgrawl: ")
	log.SetFlags(0)

	parsedURL, err := url.Parse(startURL)

	if err != nil {
		log.Fatalf("cannot parse initial URL: %v\nInitial URL: %v", err, startURL)
	}

	if parsedURL.Scheme != "" && parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		log.Fatalf("unsupported scheme: %v\nInitial URL: %v", parsedURL.Scheme, startURL)
	}

	if parsedURL.Scheme == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("\nYou did not specify the URL scheme")
		fmt.Print("Write your scheme (http/https): ")
		input, _ := reader.ReadString('\n')
		scheme := strings.ToLower(strings.TrimSpace(input))

		if scheme != "http" && scheme != "https" {
			fmt.Println("Invalid scheme input, using default: https")
			scheme = "https"
		}

		parsedURL.Scheme = scheme
	}

	fmt.Printf("\nYour initial URL is: %v\n", parsedURL.String())

	client := NewClient()
	robot := NewRobot(client.Context.ctx, client.Client)
	worker := NewWorker(*client, jobs, result, errChan, robot, wg)
	manager := NewManager(*client, jobs, result, errChan, robot)

	for i := 1; i <= numWorker; i++ {
		wg.Add(1)
		go worker.Run(i)
	}

	go manager.Run(parsedURL.String(), maxCrawlDepth)

	for err := range errChan {
		if err != nil {
			log.Printf("(WARN) %v\033[0m", err)
		} else {
			break
		}
	}

	wg.Wait()
	close(errChan)
	close(result)
	fmt.Println("finished crawling")
}
