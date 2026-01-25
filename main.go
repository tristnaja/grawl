package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/tristnaja/grawl/internal"
)

var version = "dev"

func main() {
	reader := bufio.NewReader(os.Stdin)
	var wg sync.WaitGroup
	var isDefault string
	var URLFlag string
	var startURL string
	var channelCapacity int
	var numWorker int
	var crawlDepth int
	var err error
	var versionFlag bool

	flag.StringVar(&URLFlag, "u", "", "Starting URL <shorthand>")
	flag.StringVar(&URLFlag, "url", "", "Starting URL")
	flag.BoolVar(&versionFlag, "v", false, "Print version and exit")
	flag.BoolVar(&versionFlag, "version", false, "Print version and exit")
	flag.Parse()

	if versionFlag {
		fmt.Printf("grawl version %s\n", version)
		os.Exit(0)
	}

	if URLFlag == "" && flag.NArg() <= 0 {
		fmt.Print("Enter your starting URL: ")
		URLFlag, err = reader.ReadString('\n')

		if err != nil {
			fmt.Print("Error, input again: ")
			URLFlag, err = reader.ReadString('\n')

			if err != nil {
				fmt.Println("Invalid Input, please re-run the program")
				os.Exit(1)
			}

			startURL = strings.TrimSpace(URLFlag)
		} else {
			startURL = strings.TrimSpace(URLFlag)
		}
	}

	if URLFlag == "" && flag.NArg() > 0 {
		startURL = flag.Arg(0)
	}

	if URLFlag != "" {
		startURL = strings.TrimSpace(URLFlag)
	}

	fmt.Println("\nThis is the default configuration:")
	fmt.Println("Capacity: 100")
	fmt.Println("Number of Worker(s): 10")
	fmt.Println("Crawl Depth: 2")
	fmt.Print("Do you want the default config for your worker (y/n)? ")
	isDefault, err = reader.ReadString('\n')
	isDefault = strings.ToLower(strings.TrimSpace(isDefault))

	if isDefault != "y" && isDefault != "n" && isDefault != "yes" && isDefault != "no" {
		fmt.Println("Invalid Input, please re-run the program")
		os.Exit(1)
	}

	switch isDefault {
	case "y", "yes":
		channelCapacity = 100
		numWorker = 10
		crawlDepth = 2
	case "n", "no":
		fmt.Println("Enter your configuration!")

		fmt.Print("Capacity: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		channelCapacity, err = strconv.Atoi(input)
		if err != nil {
			fmt.Println("Not a number, using default: 100")
			channelCapacity = 100
		}

		fmt.Print("Number of Worker(s): ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		numWorker, err = strconv.Atoi(input)
		if err != nil {
			fmt.Println("Not a number, using default: 10")
			channelCapacity = 10
		}

		fmt.Print("Crawl Depth: ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		crawlDepth, err = strconv.Atoi(input)
		if err != nil {
			fmt.Println("Not a number, using default: 2")
			channelCapacity = 2
		}
	}

	jobs := make(chan internal.Job, channelCapacity)
	result := make(chan internal.Result, channelCapacity)
	errChan := make(chan error, channelCapacity)

	fmt.Println("Starting Crawl with:")
	fmt.Printf("Capacity: %d\n", channelCapacity)
	fmt.Printf("Number of Worker(s): %d\n", numWorker)
	fmt.Printf("Crawl Depth: %d\n", crawlDepth)

	internal.Orchestrate(startURL, jobs, result, errChan, numWorker, crawlDepth, &wg)
}