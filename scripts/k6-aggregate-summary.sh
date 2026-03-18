#!/usr/bin/env bash
# Aggregate k6-summary.json from multiple jobs into a single comparison table.
# Usage: k6-aggregate-summary.sh <base-dir> [job1] [job2] ...
#   base-dir: directory containing one subdir per job, each with k6-summary.json
#   job names: optional list; if omitted, all subdirs of base-dir are used

set -e

usage() {
  echo "Usage: $0 <base-dir> [job1] [job2] ..." >&2
  echo "  base-dir: directory with one subdir per job, each containing k6-summary.json" >&2
  echo "  job names: optional; if omitted, every subdir is treated as a job" >&2
  exit 1
}

BASE_DIR="${1:?missing base dir}"
shift
JOBS=("$@")

if [[ ! -d "$BASE_DIR" ]]; then
  echo "Error: $BASE_DIR is not a directory" >&2
  exit 1
fi

if [[ ${#JOBS[@]} -eq 0 ]]; then
  for d in "$BASE_DIR"/*/; do
    [[ -d "$d" ]] || continue
    JOBS+=("$(basename "$d")")
  done
fi

round_ms() {
  local v="$1"
  if [[ -z "$v" || "$v" == "null" ]]; then
    echo "—"
  else
    printf "%.0f ms" "$v"
  fi
}

round_s() {
  local v="$1"
  if [[ -z "$v" || "$v" == "null" ]]; then
    echo "—"
  else
    printf "%.1f s" "$v"
  fi
}

# Collect one row per job
ROWS=()
for job in "${JOBS[@]}"; do
  f="$BASE_DIR/$job/k6-summary.json"
  if [[ ! -f "$f" ]]; then
    continue
  fi
  duration=$(jq -r '.config.duration // "null"' "$f")
  iterations=$(jq -r '
    .results.metrics[] | select(.name == "iterations") | .values.count // "null"
  ' "$f")
  trend=$(jq -r '
    .results.metrics[] | select(.name == "iteration_duration") | .values
  ' "$f")
  avg=$(jq -r '.avg // "null"' <<< "$trend")
  med=$(jq -r '.med // "null"' <<< "$trend")
  p90=$(jq -r '.p90 // "null"' <<< "$trend")
  p95=$(jq -r '.p95 // "null"' <<< "$trend")
  p99=$(jq -r '.p99 // "null"' <<< "$trend")
  max=$(jq -r '.max // "null"' <<< "$trend")

  iter_fmt="${iterations:-—}"
  [[ "$iterations" != "null" && -n "$iterations" ]] && iter_fmt=$(printf "%.0f" "$iterations")
  ROWS+=("| $job | $(round_s "$duration") | $iter_fmt | $(round_ms "$avg") | $(round_ms "$med") | $(round_ms "$p90") | $(round_ms "$p95") | $(round_ms "$p99") | $(round_ms "$max") |")
done

if [[ ${#ROWS[@]} -eq 0 ]]; then
  echo "## K6 aggregate summary"
  echo ""
  echo "No job summaries found under \`$BASE_DIR\`."
  exit 0
fi

echo "## K6 aggregate summary"
echo ""
echo "Comparison of key metrics across jobs."
echo ""
echo "| Job | Duration | Iterations | avg | med | p90 | p95 | p99 | max |"
echo "|-----|----------|------------|-----|-----|-----|-----|-----|-----|"
printf '%s\n' "${ROWS[@]}"
