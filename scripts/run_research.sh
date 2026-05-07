#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

PRESET="short"
SCENARIO="baseline"
REPEATS=1
DURATION="6m"
OUTPUT_DIR="${REPO_ROOT}/research_data"
WITH_PPROF=1
OPEN_UI=0
AUTO_KILL_PORTS=1

FIXTURE_PORT=18080
MODEL_B_METRICS_PORT=2112
MODEL_A_METRICS_PORT=2212
MODEL_B_PPROF_PORT=6060
MODEL_A_PPROF_PORT=6160

FIXTURE_PID=""
MODEL_A_PID=""
MODEL_B_PID=""

usage() {
  cat <<'EOF'
Usage:
  scripts/run_research.sh [options]

Options:
  --preset short|medium|long
  --scenario baseline|429|500
  --repeats <n>
  --duration <N[s|m|h]>
  --output-dir <path>
  --with-pprof
  --no-pprof
  --open-ui
  --auto-kill-ports
  --keep-existing-ports
  --help

Example:
  scripts/run_research.sh --preset short --scenario baseline --repeats 2 --duration 6m
EOF
}

log() {
  printf '[run_research] %s\n' "$*"
}

die() {
  printf '[run_research] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

validate_duration() {
  [[ "$DURATION" =~ ^[0-9]+[smh]$ ]] || die "invalid --duration: ${DURATION} (expected forms like 30s, 6m, 1h)"
}

cleanup_processes() {
  for pid in "$MODEL_A_PID" "$MODEL_B_PID" "$FIXTURE_PID"; do
    if [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1; then
      kill "${pid}" >/dev/null 2>&1 || true
      wait "${pid}" >/dev/null 2>&1 || true
    fi
  done
  MODEL_A_PID=""
  MODEL_B_PID=""
  FIXTURE_PID=""
}

parse_duration_seconds() {
  local value="$1"
  local num="${value%[smh]}"
  local unit="${value:${#value}-1:1}"

  case "$unit" in
    s) echo "$num" ;;
    m) echo $((num * 60)) ;;
    h) echo $((num * 3600)) ;;
    *) echo "0" ;;
  esac
}

is_pid_alive() {
  local pid="$1"
  [[ -n "$pid" ]] || return 1
  kill -0 "$pid" >/dev/null 2>&1
}

listener_pids_for_port() {
  local port="$1"
  lsof -nP -t -iTCP:"$port" -sTCP:LISTEN 2>/dev/null || true
}

ensure_ports_available() {
  local ports=($FIXTURE_PORT $MODEL_B_METRICS_PORT $MODEL_A_METRICS_PORT $MODEL_B_PPROF_PORT $MODEL_A_PPROF_PORT)

  for port in "${ports[@]}"; do
    local pids
    pids="$(listener_pids_for_port "$port")"
    [[ -z "$pids" ]] && continue

    if [[ "$AUTO_KILL_PORTS" -eq 1 ]]; then
      log "port ${port} is in use; stopping existing listeners"
      for pid in $pids; do
        kill "$pid" >/dev/null 2>&1 || true
      done
      sleep 1
      pids="$(listener_pids_for_port "$port")"
      [[ -z "$pids" ]] || die "port ${port} remains busy after cleanup"
    else
      die "port ${port} is already in use; rerun with --auto-kill-ports or stop process manually"
    fi
  done
}

wait_for_pid_or_fail() {
  local pid="$1"
  local log_file="$2"
  local name="$3"

  sleep 1
  if ! is_pid_alive "$pid"; then
    log "${name} terminated during startup"
    sed -n '1,120p' "$log_file" >&2 || true
    die "${name} failed to start"
  fi
}

trap cleanup_processes EXIT INT TERM

start_observability() {
  log "starting observability stack"
  (
    cd "${REPO_ROOT}/observability"
    docker compose up -d
  )
}

open_ui_if_requested() {
  [[ "$OPEN_UI" -eq 1 ]] || return 0

  if command -v open >/dev/null 2>&1; then
    open "http://localhost:9090" >/dev/null 2>&1 || true
    open "http://localhost:3300" >/dev/null 2>&1 || true
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open "http://localhost:9090" >/dev/null 2>&1 || true
    xdg-open "http://localhost:3300" >/dev/null 2>&1 || true
  fi
}

configure_preset() {
  case "$PRESET" in
    short)
      FIXTURE_DEPTH=7
      FIXTURE_BRANCHING=8
      FIXTURE_LATENCY="120ms"
      MODEL_DEPTH=5
      RATE_DELAY="300ms"
      MODEL_B_WORKERS=40
      MODEL_B_CAPACITY=5000
      ;;
    medium)
      FIXTURE_DEPTH=7
      FIXTURE_BRANCHING=10
      FIXTURE_LATENCY="120ms"
      MODEL_DEPTH=5
      RATE_DELAY="250ms"
      MODEL_B_WORKERS=50
      MODEL_B_CAPACITY=6000
      ;;
    long)
      FIXTURE_DEPTH=8
      FIXTURE_BRANCHING=12
      FIXTURE_LATENCY="150ms"
      MODEL_DEPTH=6
      RATE_DELAY="300ms"
      MODEL_B_WORKERS=80
      MODEL_B_CAPACITY=12000
      ;;
    *) die "invalid preset: ${PRESET}" ;;
  esac
}

configure_scenario() {
  case "$SCENARIO" in
    baseline)
      FIXTURE_STATUS_MODE="none"
      FIXTURE_STATUS_N=20
      ;;
    429)
      FIXTURE_STATUS_MODE="429_every_n"
      FIXTURE_STATUS_N=20
      ;;
    500)
      FIXTURE_STATUS_MODE="500_every_n"
      FIXTURE_STATUS_N=20
      ;;
    *) die "invalid scenario: ${SCENARIO}" ;;
  esac
}

wait_for_http() {
  local url="$1"
  local timeout_s="$2"
  local elapsed=0

  while [[ "$elapsed" -lt "$timeout_s" ]]; do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done

  return 1
}

wait_for_metrics_series() {
  local url="$1"
  local timeout_s="$2"
  local elapsed=0

  while [[ "$elapsed" -lt "$timeout_s" ]]; do
    if curl -fsS "$url" 2>/dev/null | grep -q "crawl_urls_visited_total"; then
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done

  return 1
}

start_fixture() {
  local run_dir="$1"
  log "starting fixture server"
  (
    cd "${REPO_ROOT}/fixture_server"
    go run . \
      --port "${FIXTURE_PORT}" \
      --depth "${FIXTURE_DEPTH}" \
      --branching "${FIXTURE_BRANCHING}" \
      --latency "${FIXTURE_LATENCY}" \
      --status-mode "${FIXTURE_STATUS_MODE}" \
      --status-n "${FIXTURE_STATUS_N}"
  ) >"${run_dir}/fixture.log" 2>&1 &
  FIXTURE_PID="$!"
  wait_for_pid_or_fail "$FIXTURE_PID" "${run_dir}/fixture.log" "fixture server"

  wait_for_http "http://127.0.0.1:${FIXTURE_PORT}/healthz" 30 || die "fixture server did not become ready"
}

start_model_b() {
  local run_dir="$1"
  log "starting model B worker_pool"
  (
    cd "${REPO_ROOT}/worker_pool"
    go run . \
      --url "http://127.0.0.1:${FIXTURE_PORT}/" \
      --depth "${MODEL_DEPTH}" \
      --workers "${MODEL_B_WORKERS}" \
      --capacity "${MODEL_B_CAPACITY}" \
      --rate-delay "${RATE_DELAY}" \
      --quiet \
      --metrics-addr ":${MODEL_B_METRICS_PORT}" \
      --pprof-addr ":${MODEL_B_PPROF_PORT}"
  ) >"${run_dir}/model_b.log" 2>&1 &
  MODEL_B_PID="$!"
  wait_for_pid_or_fail "$MODEL_B_PID" "${run_dir}/model_b.log" "model B"

  wait_for_http "http://127.0.0.1:${MODEL_B_METRICS_PORT}/metrics" 30 || die "model B metrics endpoint not ready"
  wait_for_metrics_series "http://127.0.0.1:${MODEL_B_METRICS_PORT}/metrics" 30 || die "model B crawl metrics not registered"
}

start_model_a() {
  local run_dir="$1"
  log "starting model A dynamic_crawler"
  (
    cd "${REPO_ROOT}/dynamic_crawler"
    go run . \
      --url "http://127.0.0.1:${FIXTURE_PORT}/" \
      --depth "${MODEL_DEPTH}" \
      --rate-delay "${RATE_DELAY}" \
      --quiet \
      --metrics-addr ":${MODEL_A_METRICS_PORT}" \
      --pprof-addr ":${MODEL_A_PPROF_PORT}"
  ) >"${run_dir}/model_a.log" 2>&1 &
  MODEL_A_PID="$!"
  wait_for_pid_or_fail "$MODEL_A_PID" "${run_dir}/model_a.log" "model A"

  wait_for_http "http://127.0.0.1:${MODEL_A_METRICS_PORT}/metrics" 30 || die "model A metrics endpoint not ready"
  wait_for_metrics_series "http://127.0.0.1:${MODEL_A_METRICS_PORT}/metrics" 30 || die "model A crawl metrics not registered"
}

prom_export() {
  local run_dir="$1"
  local start_ts="$2"
  local end_ts="$3"

  declare -A queries
  queries[visited_rate]='rate(crawl_urls_visited_total[1m])'
  queries[discovered_rate]='rate(crawl_urls_discovered_total[1m])'
  queries[fetch_error_rate]='rate(crawl_fetch_errors_total[1m])'
  queries[robots_denied_rate]='rate(crawl_robots_denied_total[1m])'
  queries[p95_fetch]='histogram_quantile(0.95, sum by (le, model) (rate(crawl_fetch_duration_seconds_bucket[1m])))'
  queries[inflight]='crawl_inflight_requests'
  queries[workers]='crawl_active_workers'
  queries[goroutines]='crawl_active_goroutines'
  queries[rss]='process_resident_memory_bytes'
  queries[heap_alloc]='go_memstats_heap_alloc_bytes'
  queries[runtime_goroutines]='go_goroutines'
  queries[gc_pause_p99]='go_gc_duration_seconds{quantile="0.99"}'

  for key in "${!queries[@]}"; do
    curl -fsSG 'http://localhost:9090/api/v1/query_range' \
      --data-urlencode "query=${queries[$key]}" \
      --data-urlencode "start=${start_ts}" \
      --data-urlencode "end=${end_ts}" \
      --data-urlencode 'step=5s' \
      -o "${run_dir}/prom_${key}.json"
  done
}

capture_pprof() {
  local run_dir="$1"
  [[ "$WITH_PPROF" -eq 1 ]] || return 0

  log "capturing pprof snapshots"
  mkdir -p "$run_dir"

  if wait_for_http "http://127.0.0.1:${MODEL_B_PPROF_PORT}/debug/pprof/" 2; then
    go tool pprof -proto "http://127.0.0.1:${MODEL_B_PPROF_PORT}/debug/pprof/profile?seconds=20" >"${run_dir}/model_b_cpu.pb.gz" || true
    go tool pprof -proto "http://127.0.0.1:${MODEL_B_PPROF_PORT}/debug/pprof/goroutine" >"${run_dir}/model_b_goroutine.pb.gz" || true
  else
    log "model B pprof unavailable; skipping"
  fi

  if wait_for_http "http://127.0.0.1:${MODEL_A_PPROF_PORT}/debug/pprof/" 2; then
    go tool pprof -proto "http://127.0.0.1:${MODEL_A_PPROF_PORT}/debug/pprof/profile?seconds=20" >"${run_dir}/model_a_cpu.pb.gz" || true
    go tool pprof -proto "http://127.0.0.1:${MODEL_A_PPROF_PORT}/debug/pprof/goroutine" >"${run_dir}/model_a_goroutine.pb.gz" || true
  else
    log "model A pprof unavailable; skipping"
  fi
}

ensure_run_log_header() {
  local run_log="$1"
  if [[ ! -f "$run_log" ]]; then
    cat >"$run_log" <<'EOF'
run_id,preset,scenario,start_ts,end_ts,duration,fixture_depth,fixture_branching,fixture_latency,fixture_status_mode,fixture_status_n,crawl_depth,rate_delay,model_b_workers,model_b_capacity,run_dir,status
EOF
  fi
}

append_run_log() {
  local run_log="$1"
  local run_id="$2"
  local start_ts="$3"
  local end_ts="$4"
  local run_dir="$5"
  local status="$6"

  printf '%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n' \
    "$run_id" "$PRESET" "$SCENARIO" "$start_ts" "$end_ts" "$DURATION" \
    "$FIXTURE_DEPTH" "$FIXTURE_BRANCHING" "$FIXTURE_LATENCY" "$FIXTURE_STATUS_MODE" "$FIXTURE_STATUS_N" \
    "$MODEL_DEPTH" "$RATE_DELAY" "$MODEL_B_WORKERS" "$MODEL_B_CAPACITY" "$run_dir" "$status" >>"$run_log"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --preset) PRESET="$2"; shift 2 ;;
      --scenario) SCENARIO="$2"; shift 2 ;;
      --repeats) REPEATS="$2"; shift 2 ;;
      --duration) DURATION="$2"; shift 2 ;;
      --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
      --with-pprof) WITH_PPROF=1; shift ;;
      --no-pprof) WITH_PPROF=0; shift ;;
      --open-ui) OPEN_UI=1; shift ;;
      --auto-kill-ports) AUTO_KILL_PORTS=1; shift ;;
      --keep-existing-ports) AUTO_KILL_PORTS=0; shift ;;
      --help|-h) usage; exit 0 ;;
      *) die "unknown argument: $1" ;;
    esac
  done
}

main() {
  parse_args "$@"

  require_cmd docker
  require_cmd go
  require_cmd curl
  [[ -d "${REPO_ROOT}/observability" ]] || die "missing observability directory"
  [[ -d "${REPO_ROOT}/fixture_server" ]] || die "missing fixture_server directory"
  [[ -d "${REPO_ROOT}/worker_pool" ]] || die "missing worker_pool directory"
  [[ -d "${REPO_ROOT}/dynamic_crawler" ]] || die "missing dynamic_crawler directory"
  validate_duration
  [[ "$REPEATS" =~ ^[0-9]+$ ]] || die "--repeats must be a positive integer"
  [[ "$REPEATS" -gt 0 ]] || die "--repeats must be > 0"

  configure_preset
  configure_scenario
  ensure_ports_available

  mkdir -p "$OUTPUT_DIR"
  local run_log="${OUTPUT_DIR}/run_log.csv"
  ensure_run_log_header "$run_log"

  start_observability
  open_ui_if_requested

  for i in $(seq 1 "$REPEATS"); do
    local run_id
    run_id="$(printf '%s_%s_r%02d' "$PRESET" "$SCENARIO" "$i")"
    local run_dir="${OUTPUT_DIR}/${run_id}"
    mkdir -p "$run_dir"

    log "starting run ${run_id}"
    start_fixture "$run_dir"
    start_model_b "$run_dir"
    start_model_a "$run_dir"

    local start_ts
    start_ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

    local duration_s
    duration_s="$(parse_duration_seconds "$DURATION")"
    local elapsed=0
    local run_status="ok"
    while [[ "$elapsed" -lt "$duration_s" ]]; do
      if ! is_pid_alive "$MODEL_A_PID" && ! is_pid_alive "$MODEL_B_PID"; then
        run_status="completed_early"
        break
      fi
      sleep 1
      elapsed=$((elapsed + 1))
    done

    local end_ts
    end_ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

    capture_pprof "$run_dir"
    prom_export "$run_dir" "$start_ts" "$end_ts"

    cleanup_processes
    append_run_log "$run_log" "$run_id" "$start_ts" "$end_ts" "$run_dir" "$run_status"
    log "completed run ${run_id}"
    sleep 2
  done

  if command -v python3 >/dev/null 2>&1; then
    python3 "${REPO_ROOT}/scripts/summarize_research.py" --input-dir "$OUTPUT_DIR" || true
  fi

  log "all runs complete"
}

main "$@"
