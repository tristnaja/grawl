# grawl Research Branch

This repository layout is prepared for **crawler architecture research**, not for packaging or distribution.

## Purpose of This Branch

- Compare two crawler implementations under a common research project.
- Keep both implementations runnable locally.
- Prepare the codebase for future metrics/observability experiments.

## Implementations

- `worker_pool/` - **Model B** (fixed worker-pool architecture).
- `dynamic_crawler/` - **Model A** (dynamic goroutine-per-task architecture).

Each folder is an independent Go module and has its own README.

## Research Model Fit Assessment

Based on `RESEARCH_OUTLINE.md`:

- **Model A criteria:** dynamic recursion where each discovered link triggers immediate `go func()` and scheduling is delegated to Go runtime.
- **Model B criteria:** static fixed-size worker pool with buffered job queue where only `N` tasks run concurrently.

Assessment result:

- `worker_pool/` **fits Model B** as required. It uses a fixed worker pool (`numWorker`), channel-based job queue, and manager-driven dispatch.
- `dynamic_crawler/` fits Model A criteria using recursive goroutine-per-task scheduling.

Final mapping in this branch:

- **Model A:** `dynamic_crawler/`
- **Model B:** `worker_pool/`

## Local Usage

Run commands from each implementation folder.

### Model A (dynamic)

```bash
cd dynamic_crawler
go run . --url https://example.com
```

### Model B (worker pool)

```bash
cd worker_pool
go run . --url https://example.com
```

## What Changed for Research Setup

- Original crawler moved under `worker_pool/`.
- New second implementation created under `dynamic_crawler/`.
- Goreleaser/release workflow removed for local-only research use.

## Deferred Work

- Benchmark and load-test scenarios.
- Data collection and analysis scripts.

## Experiment Mode and Observability

Both models now support non-interactive experiment flags and optional observability endpoints.

- Common experiment flags:
  - `--url`
  - `--depth`
  - `--rate-delay`
  - `--quiet`
  - `--metrics-addr`
  - `--pprof-addr`
- Model B only:
  - `--workers`
  - `--capacity`

Local stack:

- Prometheus + Grafana config lives in `observability/`.
- Full reproducible procedure is in `RESEARCH_RUNBOOK.md`.
- Full step-by-step data reproduction workflow is in `DATA_REPRODUCTION_GUIDE.md`.

## Local Fixture Target

Use `fixture_server/` as the deterministic crawl target for reproducible experiments.

- Example: `go run ./fixture_server --port 18080 --depth 4 --branching 5 --latency 50ms`
