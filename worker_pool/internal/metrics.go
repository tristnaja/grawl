package internal

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsRegistered bool

	metricURLsVisited    *prometheus.CounterVec
	metricURLsAllowed    *prometheus.CounterVec
	metricURLsDiscovered *prometheus.CounterVec
	metricFetchErrors    *prometheus.CounterVec
	metricRobotsDenied   *prometheus.CounterVec
	metricRetries        *prometheus.CounterVec

	metricFetchDuration *prometheus.HistogramVec

	metricInflightRequests *prometheus.GaugeVec
	metricActiveWorkers    *prometheus.GaugeVec
	metricQueueDepth       *prometheus.GaugeVec
)

func RegisterMetrics(registry prometheus.Registerer, model string) error {
	if metricsRegistered {
		return nil
	}

	labels := prometheus.Labels{"model": model}

	metricURLsVisited = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "crawl_urls_visited_total",
		Help:        "Total unique URLs visited by scheduler.",
		ConstLabels: labels,
	}, []string{})

	metricURLsAllowed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "crawl_urls_allowed_total",
		Help:        "Total URLs allowed by robots rules.",
		ConstLabels: labels,
	}, []string{})

	metricURLsDiscovered = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "crawl_urls_discovered_total",
		Help:        "Total discovered URLs from parsed documents.",
		ConstLabels: labels,
	}, []string{})

	metricFetchErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "crawl_fetch_errors_total",
		Help:        "Total fetch/parsing errors while crawling.",
		ConstLabels: labels,
	}, []string{})

	metricRobotsDenied = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "crawl_robots_denied_total",
		Help:        "Total URLs denied by robots rules.",
		ConstLabels: labels,
	}, []string{})

	metricRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "crawl_retries_total",
		Help:        "Total retry attempts performed by HTTP fetch.",
		ConstLabels: labels,
	}, []string{})

	metricFetchDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "crawl_fetch_duration_seconds",
		Help:        "Duration of fetch operations in seconds.",
		ConstLabels: labels,
		Buckets:     prometheus.DefBuckets,
	}, []string{})

	metricInflightRequests = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "crawl_inflight_requests",
		Help:        "Current number of in-flight HTTP requests.",
		ConstLabels: labels,
	}, []string{})

	metricActiveWorkers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "crawl_active_workers",
		Help:        "Current number of active workers processing jobs.",
		ConstLabels: labels,
	}, []string{})

	metricQueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "crawl_queue_depth",
		Help:        "Current depth of worker queue.",
		ConstLabels: labels,
	}, []string{})

	for _, collector := range []prometheus.Collector{
		metricURLsVisited,
		metricURLsAllowed,
		metricURLsDiscovered,
		metricFetchErrors,
		metricRobotsDenied,
		metricRetries,
		metricFetchDuration,
		metricInflightRequests,
		metricActiveWorkers,
		metricQueueDepth,
	} {
		if err := registry.Register(collector); err != nil {
			return fmt.Errorf("registering collector: %w", err)
		}
	}

	metricsRegistered = true
	return nil
}

func MetricURLsVisitedInc() {
	if metricURLsVisited != nil {
		metricURLsVisited.WithLabelValues().Inc()
	}
}

func MetricURLsAllowedInc() {
	if metricURLsAllowed != nil {
		metricURLsAllowed.WithLabelValues().Inc()
	}
}

func MetricURLsDiscoveredAdd(count int) {
	if metricURLsDiscovered != nil {
		metricURLsDiscovered.WithLabelValues().Add(float64(count))
	}
}

func MetricFetchErrorsInc() {
	if metricFetchErrors != nil {
		metricFetchErrors.WithLabelValues().Inc()
	}
}

func MetricRobotsDeniedInc() {
	if metricRobotsDenied != nil {
		metricRobotsDenied.WithLabelValues().Inc()
	}
}

func MetricRetriesInc() {
	if metricRetries != nil {
		metricRetries.WithLabelValues().Inc()
	}
}

func MetricFetchDurationObserve(seconds float64) {
	if metricFetchDuration != nil {
		metricFetchDuration.WithLabelValues().Observe(seconds)
	}
}

func MetricInflightRequestsInc() {
	if metricInflightRequests != nil {
		metricInflightRequests.WithLabelValues().Inc()
	}
}

func MetricInflightRequestsDec() {
	if metricInflightRequests != nil {
		metricInflightRequests.WithLabelValues().Dec()
	}
}

func MetricActiveWorkersInc() {
	if metricActiveWorkers != nil {
		metricActiveWorkers.WithLabelValues().Inc()
	}
}

func MetricActiveWorkersDec() {
	if metricActiveWorkers != nil {
		metricActiveWorkers.WithLabelValues().Dec()
	}
}

func MetricQueueDepthSet(depth int) {
	if metricQueueDepth != nil {
		metricQueueDepth.WithLabelValues().Set(float64(depth))
	}
}
