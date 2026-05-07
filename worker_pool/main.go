package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	var interactive bool
	var quiet bool
	var metricsAddr string
	var pprofAddr string
	var rateDelay time.Duration
	var capacityFlag int
	var workersFlag int
	var depthFlag int

	flag.StringVar(&URLFlag, "u", "", "Starting URL <shorthand>")
	flag.StringVar(&URLFlag, "url", "", "Starting URL")
	flag.BoolVar(&versionFlag, "v", false, "Print version and exit")
	flag.BoolVar(&versionFlag, "version", false, "Print version and exit")
	flag.BoolVar(&interactive, "interactive", false, "Use interactive configuration")
	flag.BoolVar(&quiet, "quiet", false, "Minimize crawler output")
	flag.StringVar(&metricsAddr, "metrics-addr", "", "Expose Prometheus metrics on address (e.g. :2112)")
	flag.StringVar(&pprofAddr, "pprof-addr", "", "Expose pprof server on address (e.g. :6060)")
	flag.DurationVar(&rateDelay, "rate-delay", 2*time.Second, "Default per-host crawl delay when robots does not define one")
	flag.IntVar(&capacityFlag, "capacity", 100, "Job/result channel capacity")
	flag.IntVar(&workersFlag, "workers", 10, "Number of worker goroutines")
	flag.IntVar(&depthFlag, "depth", 2, "Maximum crawl depth")
	flag.Parse()

	if versionFlag {
		fmt.Printf("grawl version %s\n", version)
		os.Exit(0)
	}

	if URLFlag == "" && flag.NArg() <= 0 && interactive {
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

	if !interactive {
		channelCapacity = capacityFlag
		numWorker = workersFlag
		crawlDepth = depthFlag
	} else {
		fmt.Println("\nThis is the default configuration:")
		fmt.Println("Capacity: 100")
		fmt.Println("Number of Worker(s): 10")
		fmt.Println("Crawl Depth: 2")
		fmt.Print("Do you want the default config for your worker (y/n)? ")
		isDefault, err = reader.ReadString('\n')
		useDefault, parseErr := parseYesNo(isDefault)
		if err != nil || parseErr != nil {
			fmt.Println("Invalid Input, please re-run the program")
			os.Exit(1)
		}

		if useDefault {
			channelCapacity = 100
			numWorker = 10
			crawlDepth = 2
		} else {
			fmt.Println("Enter your configuration!")

			fmt.Print("Capacity: ")
			input, _ := reader.ReadString('\n')
			channelCapacity = parsePositiveIntOrDefault(input, 100)
			if strings.TrimSpace(input) != "" && channelCapacity == 100 {
				fmt.Println("Not a number, using default: 100")
			}

			fmt.Print("Number of Worker(s): ")
			input, _ = reader.ReadString('\n')
			numWorker = parsePositiveIntOrDefault(input, 10)
			if strings.TrimSpace(input) != "" && numWorker == 10 {
				fmt.Println("Not a number, using default: 10")
			}

			fmt.Print("Crawl Depth: ")
			input, _ = reader.ReadString('\n')
			crawlDepth = parsePositiveIntOrDefault(input, 2)
			if strings.TrimSpace(input) != "" && crawlDepth == 2 {
				fmt.Println("Not a number, using default: 2")
			}
		}
	}

	if startURL == "" {
		fmt.Println("starting URL is required (use --url or positional argument)")
		os.Exit(1)
	}

	internal.SetQuietMode(quiet)
	internal.SetDefaultRateDelay(rateDelay)
	if metricsAddr != "" {
		if err := startMetricsServer(metricsAddr); err != nil {
			fmt.Printf("failed to start metrics server: %v\n", err)
			os.Exit(1)
		}
	}

	if pprofAddr != "" {
		go func() {
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				log.Printf("pprof server stopped: %v", err)
			}
		}()
	}

	jobs := make(chan internal.Job, channelCapacity)
	result := make(chan internal.Result, channelCapacity)
	errChan := make(chan error, channelCapacity)

	fmt.Println("Starting Crawl with:")
	fmt.Printf("Capacity: %d\n", channelCapacity)
	fmt.Printf("Number of Worker(s): %d\n", numWorker)
	fmt.Printf("Crawl Depth: %d\n", crawlDepth)

	if err := internal.Orchestrate(startURL, jobs, result, errChan, numWorker, crawlDepth, &wg); err != nil {
		fmt.Printf("failed to orchestrate crawl: %v\n", err)
		os.Exit(1)
	}
}

func parseYesNo(input string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch normalized {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, errors.New("invalid yes/no input")
	}
}

func parsePositiveIntOrDefault(input string, fallback int) int {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return fallback
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}

func startMetricsServer(addr string) error {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if err := internal.RegisterMetrics(registry, "model_b_worker_pool"); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("metrics server stopped: %v", err)
		}
	}()

	return nil
}
