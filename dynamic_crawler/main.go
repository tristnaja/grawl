package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tristnaja/grawl/dynamic_crawler/internal"
)

var version = "dev"

func main() {
	startURL, maxDepth, config, err := readConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config.MaxDepth = maxDepth

	if config.MetricsAddr != "" {
		if err := startMetricsServer(config.MetricsAddr); err != nil {
			log.Fatalf("failed to start metrics server: %v", err)
		}
	}

	if config.PProfAddr != "" {
		go func() {
			if err := http.ListenAndServe(config.PProfAddr, nil); err != nil {
				log.Printf("pprof server stopped: %v", err)
			}
		}()
	}

	internal.SetQuietMode(config.Quiet)

	crawler := internal.NewCrawler(config)
	result, err := crawler.Crawl(ctx, startURL)
	if err != nil {
		log.Fatalf("crawl failed: %v", err)
	}

	fmt.Printf("\nCrawl completed\n")
	fmt.Printf("Visited URLs: %d\n", result.Visited)
	fmt.Printf("Allowed URLs: %d\n", result.Allowed)
	fmt.Printf("Discovered URLs: %d\n", result.Discovered)
}

func readConfig() (string, int, internal.Config, error) {
	var urlFlag string
	var versionFlag bool
	var interactive bool
	var quiet bool
	var metricsAddr string
	var pprofAddr string
	var depthFlag int
	var userAgent string
	var httpTimeout time.Duration
	var retryCount int
	var retryBackoff time.Duration
	var rateDelay time.Duration

	flag.StringVar(&urlFlag, "u", "", "Starting URL (shorthand)")
	flag.StringVar(&urlFlag, "url", "", "Starting URL")
	flag.BoolVar(&versionFlag, "v", false, "Print version and exit")
	flag.BoolVar(&versionFlag, "version", false, "Print version and exit")
	flag.BoolVar(&interactive, "interactive", false, "Use interactive configuration")
	flag.BoolVar(&quiet, "quiet", false, "Minimize crawler output")
	flag.StringVar(&metricsAddr, "metrics-addr", "", "Expose Prometheus metrics on address (e.g. :2212)")
	flag.StringVar(&pprofAddr, "pprof-addr", "", "Expose pprof server on address (e.g. :6160)")
	flag.IntVar(&depthFlag, "depth", 2, "Maximum crawl depth")
	flag.StringVar(&userAgent, "user-agent", "Grawl-Dynamic/1.0", "Crawler user agent")
	flag.DurationVar(&httpTimeout, "http-timeout", 30*time.Second, "HTTP client timeout")
	flag.IntVar(&retryCount, "retry-count", 3, "Number of retries for fetch failures")
	flag.DurationVar(&retryBackoff, "retry-backoff", 2*time.Second, "Base retry backoff")
	flag.DurationVar(&rateDelay, "rate-delay", 2*time.Second, "Default per-host crawl delay when robots does not define one")
	flag.Parse()

	if versionFlag {
		fmt.Printf("semaphore crawler version %s\n", version)
		os.Exit(0)
	}

	startURL := strings.TrimSpace(urlFlag)
	if startURL == "" && flag.NArg() > 0 {
		startURL = strings.TrimSpace(flag.Arg(0))
	}

	reader := bufio.NewReader(os.Stdin)
	if startURL == "" && interactive {
		fmt.Print("Enter your starting URL: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", 0, internal.Config{}, fmt.Errorf("reading URL input: %w", err)
		}

		startURL = strings.TrimSpace(input)
	}

	if startURL == "" {
		return "", 0, internal.Config{}, fmt.Errorf("starting URL cannot be empty")
	}

	baseConfig := internal.Config{
		UserAgent:    userAgent,
		HTTPTimeout:  httpTimeout,
		RetryCount:   retryCount,
		RetryBackoff: retryBackoff,
		BaseDelay:    rateDelay,
		Quiet:        quiet,
		MetricsAddr:  metricsAddr,
		PProfAddr:    pprofAddr,
	}

	if !interactive {
		return startURL, depthFlag, baseConfig, nil
	}

	fmt.Println("\nDefault configuration:")
	fmt.Println("Crawl Depth: 2")
	fmt.Print("Use defaults? (y/n): ")

	confirmInput, err := reader.ReadString('\n')
	if err != nil {
		return "", 0, internal.Config{}, fmt.Errorf("reading confirmation input: %w", err)
	}

	confirm := strings.ToLower(strings.TrimSpace(confirmInput))
	if confirm == "y" || confirm == "yes" {
		return startURL, 2, baseConfig, nil
	}

	if confirm != "n" && confirm != "no" {
		return "", 0, internal.Config{}, fmt.Errorf("expected y/n answer")
	}

	maxDepth, err := readInt(reader, "Crawl Depth", 2)
	if err != nil {
		return "", 0, internal.Config{}, err
	}

	return startURL, maxDepth, baseConfig, nil
}

func readInt(reader *bufio.Reader, label string, fallback int) (int, error) {
	fmt.Printf("%s: ", label)
	input, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", label, err)
	}

	valueText := strings.TrimSpace(input)
	if valueText == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(valueText)
	if err != nil {
		return fallback, nil
	}

	if value <= 0 {
		return fallback, nil
	}

	return value, nil
}

func startMetricsServer(addr string) error {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if err := internal.RegisterMetrics(registry, "model_a_dynamic"); err != nil {
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
