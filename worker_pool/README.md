# Worker Pool Crawler (Baseline)

This folder contains the original `grawl` implementation, preserved as the **worker-pool baseline** for research comparison.

## Purpose

- Serve as version A in crawler architecture comparisons.
- Preserve original behavior and structure as-is.
- Provide a stable baseline before running instrumentation or benchmark experiments.

## Architecture

- Worker pool with channels (`jobs`, `result`, `errChan`).
- `Manager` coordinates queueing and crawl depth.
- `Worker` fetches and parses pages.
- `Scheduler` prevents duplicate URL visits.
- `Robots` handles robots.txt parsing and host-specific rate limiting.

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
  --workers 10 \
  --capacity 100 \
  --rate-delay 200ms \
  --quiet \
  --metrics-addr :2112 \
  --pprof-addr :6060
```

## Build and Verify

```bash
go build ./...
go test ./...
go vet ./...
```

## Notes for Research

- No benchmark/data collection setup is enabled yet.
- No Prometheus/Grafana integration is configured yet.
- Keep this baseline unchanged unless explicitly needed for bug fixes.
