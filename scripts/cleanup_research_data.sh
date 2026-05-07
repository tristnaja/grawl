#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

TARGET_DIR="${REPO_ROOT}/research_data"
DRY_RUN=0
ASSUME_YES=0

usage() {
  cat <<'EOF'
Usage:
  scripts/cleanup_research_data.sh [options]

Options:
  --path <dir>   Target directory to clean (default: ./research_data)
  --dry-run      Show what would be removed
  --yes          Skip confirmation prompt
  --help         Show this message

This script removes all contents inside the target research data directory
while keeping the directory itself.
EOF
}

log() {
  printf '[cleanup_research_data] %s\n' "$*"
}

die() {
  printf '[cleanup_research_data] ERROR: %s\n' "$*" >&2
  exit 1
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --path)
        TARGET_DIR="$2"
        shift 2
        ;;
      --dry-run)
        DRY_RUN=1
        shift
        ;;
      --yes)
        ASSUME_YES=1
        shift
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done
}

normalize_path() {
  local path="$1"
  python3 - <<'PY' "$path"
import os
import sys

print(os.path.realpath(sys.argv[1]))
PY
}

verify_target() {
  command -v python3 >/dev/null 2>&1 || die "missing required command: python3"

  TARGET_DIR="$(normalize_path "$TARGET_DIR")"
  local repo_research_dir
  repo_research_dir="$(normalize_path "${REPO_ROOT}/research_data")"

  [[ -d "$TARGET_DIR" ]] || die "target directory does not exist: ${TARGET_DIR}"
  [[ "$TARGET_DIR" != "/" ]] || die "refusing to clean root directory"
  [[ "$TARGET_DIR" == "$repo_research_dir" ]] || die "refusing to clean non-research directory: ${TARGET_DIR}"
}

confirm_if_needed() {
  [[ "$ASSUME_YES" -eq 1 ]] && return 0
  [[ "$DRY_RUN" -eq 1 ]] && return 0

  if [[ -t 0 ]]; then
    printf 'Remove all contents in %s? [y/N]: ' "$TARGET_DIR"
    read -r reply
    [[ "$reply" == "y" || "$reply" == "Y" ]] || die "aborted"
    return 0
  fi

  die "non-interactive mode requires --yes"
}

list_items() {
  find "$TARGET_DIR" -mindepth 1 -maxdepth 1 -print
}

cleanup() {
  local items
  items="$(list_items || true)"

  if [[ -z "$items" ]]; then
    log "nothing to clean in ${TARGET_DIR}"
    return 0
  fi

  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "dry run; would remove:"
    printf '%s\n' "$items"
    return 0
  fi

  find "$TARGET_DIR" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
  log "cleanup complete: ${TARGET_DIR}"
}

main() {
  parse_args "$@"
  verify_target
  confirm_if_needed
  cleanup
}

main "$@"
