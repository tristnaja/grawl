# grawl Fixture Server

The fixture server is a deterministic local website used to benchmark `dynamic_crawler` (Model A) and `worker_pool` (Model B) under controlled, repeatable conditions.

## Why this exists

Public websites are poor benchmarking targets because they introduce:

- request throttling (`429`)
- variable latency
- changing content/structure
- unknown robots policies over time

This server gives you a stable crawl graph and predictable behavior so runs are reproducible.

## What it serves

- `/healthz` - health check (`200`, body `ok`)
- `/robots.txt` - allow-all robots policy
- `/` - root HTML page with deterministic child links
- `/node/{level}/{id}` - deterministic graph nodes

## Core behavior controls

- `--depth` controls graph levels.
- `--branching` controls links per node.
- `--latency` adds fixed delay to every response.

Phase 2 support (reliability experiments):

- `--status-mode` for deterministic error injection:
  - `none`
  - `429_every_n`
  - `500_every_n`
- `--status-n` controls injection interval.

## Quick start

```bash
cd /Users/tristan/dev/projects/go/grawl/fixture_server
go run . --port 18080 --depth 4 --branching 5 --latency 50ms
```

Verify:

```bash
curl -i http://localhost:18080/healthz
curl -i http://localhost:18080/robots.txt
curl -i http://localhost:18080/
```

Use this target URL for both crawlers:

- `http://127.0.0.1:18080/`

## Phase 2 error-injection examples

Inject 429 on every 20th request:

```bash
go run . --port 18080 --depth 4 --branching 5 --latency 50ms --status-mode 429_every_n --status-n 20
```

Inject 500 on every 20th request:

```bash
go run . --port 18080 --depth 4 --branching 5 --latency 50ms --status-mode 500_every_n --status-n 20
```

## Notes

- Keep fixture settings fixed across repeated runs in one scenario.
- For A/B fairness, use identical fixture config and crawler depth/rate-delay.
