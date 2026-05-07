#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUN_SCRIPT="${SCRIPT_DIR}/run_research.sh"

PRESET="short"
REPEATS=1
DURATION="6m"
OUTPUT_DIR=""
WITH_PPROF_FLAG="--with-pprof"
OPEN_UI=0

usage() {
  cat <<'EOF'
Usage:
  scripts/run_all_phase2.sh [options]

Options:
  --preset short|medium|long
  --repeats <n>
  --duration <N[s|m|h]>
  --output-dir <path>
  --with-pprof
  --no-pprof
  --open-ui
  --help

This script runs scenarios sequentially:
  baseline -> 429 -> 500
EOF
}

die() {
  printf '[run_all_phase2] ERROR: %s\n' "$*" >&2
  exit 1
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --preset) PRESET="$2"; shift 2 ;;
      --repeats) REPEATS="$2"; shift 2 ;;
      --duration) DURATION="$2"; shift 2 ;;
      --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
      --with-pprof) WITH_PPROF_FLAG="--with-pprof"; shift ;;
      --no-pprof) WITH_PPROF_FLAG="--no-pprof"; shift ;;
      --open-ui) OPEN_UI=1; shift ;;
      --help|-h) usage; exit 0 ;;
      *) die "unknown argument: $1" ;;
    esac
  done
}

run_scenario() {
  local scenario="$1"
  local open_ui_args=()
  local output_arg=()
  if [[ "$OPEN_UI" -eq 1 && "$scenario" == "baseline" ]]; then
    open_ui_args=(--open-ui)
  fi

  if [[ -n "$OUTPUT_DIR" ]]; then
    output_arg=(--output-dir "$OUTPUT_DIR")
  fi

  "$RUN_SCRIPT" \
    --preset "$PRESET" \
    --scenario "$scenario" \
    --repeats "$REPEATS" \
    --duration "$DURATION" \
    "$WITH_PPROF_FLAG" \
    "${open_ui_args[@]}" \
    "${output_arg[@]}"
}

main() {
  parse_args "$@"

  [[ -x "$RUN_SCRIPT" ]] || die "missing executable run script: $RUN_SCRIPT"

  run_scenario baseline
  sleep 3
  run_scenario 429
  sleep 3
  run_scenario 500

  printf '[run_all_phase2] Completed baseline, 429, and 500 scenarios.\n'
}

main "$@"
