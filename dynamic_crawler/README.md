# Dynamic Crawler (Model A)

This folder contains **Model A** from the research outline.

## Mapping to Research Outline

- Type: **Dynamic**
- Concurrency model: **Goroutine-per-task recursive crawl**
- Scheduling approach: relies on the **Go runtime scheduler** instead of a fixed worker pool

## Key Design Choices

- Every discovered link spawns a new `go` task (bounded only by crawl depth and dedupe).
- No central fixed-size worker pool or job queue limit.
- Shared state (visited links and robots cache) is synchronized with mutexes.
- HTTP requests include timeout and retry with backoff.

## Run Locally

From this folder:

```bash
go run .
go run . --url https://example.com
go run . --version
```

### Non-interactive experiment mode

```bash
go run . \
  --url https://example.com \
  --depth 2 \
  --rate-delay 200ms \
  --quiet \
  --metrics-addr :2212 \
  --pprof-addr :6160
```

## Build and Verify

```bash
go mod tidy
go build ./...
go test ./...
go vet ./...
```

## Current Scope

- Built for local research experiments.
- Metrics/tracing integration is deferred to a later phase.
