package internal

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
)

func Orchestrate(startURL string, jobs chan Job, result chan Result, errChan chan error, numWorker int, maxCrawlDepth int, wg *sync.WaitGroup) error {
	log.SetPrefix("\033[33mgrawl: ")
	log.SetFlags(0)

	parsedURL, err := validateStartURL(startURL)
	if err != nil {
		return err
	}

	if parsedURL.Scheme == "" {
		scheme := promptScheme()
		parsedURL.Scheme = normalizeSchemeInput(scheme)
	}

	finalURL, err := buildFinalURL(parsedURL)
	if err != nil {
		return err
	}

	fmt.Printf("\nYour initial URL is: %v\n", finalURL)

	client := NewClient()
	robot := NewRobot(client.Context.ctx, client.Client)
	worker := NewWorker(*client, jobs, result, errChan, robot, wg)
	manager := NewManager(*client, jobs, result, errChan, robot)

	for i := 1; i <= numWorker; i++ {
		wg.Add(1)
		go worker.Run(i)
	}

	go manager.Run(finalURL, maxCrawlDepth)

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

	return nil
}

func validateStartURL(startURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(startURL)

	if err != nil {
		return nil, fmt.Errorf("cannot parse initial URL: %w", err)
	}

	if parsedURL.Scheme != "" && parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return nil, errors.New("initial URL must include host")
	}

	return parsedURL, nil
}

func promptScheme() string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\nYou did not specify the URL scheme")
	fmt.Print("Write your scheme (http/https): ")
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func normalizeSchemeInput(input string) string {
	scheme := strings.ToLower(strings.TrimSpace(input))
	if scheme == "http" || scheme == "https" {
		return scheme
	}

	fmt.Println("Invalid scheme input, using default: https")
	return "https"
}

func buildFinalURL(parsedURL *url.URL) (string, error) {
	finalURL := parsedURL.String()
	if finalURL == "" {
		return "", errors.New("initial URL cannot be empty")
	}

	return finalURL, nil
}
