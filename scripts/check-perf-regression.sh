#!/usr/bin/env bash
# Gate go test -bench -benchmem output against fixed allocation ceilings.
# Usage: check-perf-regression.sh <bench-output-file> <thresholds-file>
#
# <bench-output-file> is the raw stdout of a `go test -bench=. -benchmem` run.
# <thresholds-file> has one gated benchmark per non-comment, non-blank line:
#   <benchmark-name> <max_allocs_per_op> <max_bytes_per_op>
#
# Exit codes:
#   0 - every gated benchmark is within its allocs/op and bytes/op ceiling
#   1 - a gated benchmark exceeds a ceiling, or is missing from the bench
#       output entirely (renamed/deleted benchmark)
#
# Benchmarks present in the bench output but absent from the thresholds file
# are ignored (not gated) so exploratory, non-blocking benchmarks can coexist.

set -euo pipefail

usage() {
  echo "Usage: $0 <bench-output-file> <thresholds-file>" >&2
  exit 1
}

BENCH_FILE="${1:?missing bench-output-file}"
THRESHOLDS_FILE="${2:?missing thresholds-file}"

[[ -f "$BENCH_FILE" ]] || { echo "Error: $BENCH_FILE not found" >&2; exit 1; }
[[ -f "$THRESHOLDS_FILE" ]] || { echo "Error: $THRESHOLDS_FILE not found" >&2; exit 1; }

# Parse bench output into a lookup file: "<name> <allocs/op> <bytes/op>".
# A benchmark name looks like "BenchmarkFoo-12" or "BenchmarkFoo/bar-1-12";
# only the trailing "-<GOMAXPROCS>" is stripped, not any "-<N>" that is part
# of a sub-benchmark's own name.
MEASURED=$(mktemp)
trap 'rm -f "$MEASURED"' EXIT

awk '
  /^Benchmark/ {
    name = $1
    sub(/-[0-9]+$/, "", name)
    allocs = $(NF-1)
    bytes = $(NF-3)
    print name, allocs, bytes
  }
' "$BENCH_FILE" > "$MEASURED"

FAILURES=()
declare -a TABLE_ROWS=()
OVERALL_STATUS="pass"

while IFS= read -r line; do
  # strip comments and blank lines
  line="${line%%#*}"
  line="$(echo "$line" | xargs)"
  [[ -z "$line" ]] && continue

  read -r name max_allocs max_bytes <<< "$line"

  measured_line=$(awk -v n="$name" '$1 == n { print; exit }' "$MEASURED")
  if [[ -z "$measured_line" ]]; then
    FAILURES+=("$name: MISSING from bench output (benchmark renamed or deleted?)")
    TABLE_ROWS+=("| $name | — | — | $max_allocs | $max_bytes | MISSING |")
    OVERALL_STATUS="fail"
    continue
  fi

  read -r _ measured_allocs measured_bytes <<< "$measured_line"

  status="pass"
  if (( measured_allocs > max_allocs )); then
    FAILURES+=("$name: allocs/op $measured_allocs exceeds ceiling $max_allocs")
    status="fail"
  fi
  if (( measured_bytes > max_bytes )); then
    FAILURES+=("$name: B/op $measured_bytes exceeds ceiling $max_bytes")
    status="fail"
  fi
  if [[ "$status" == "fail" ]]; then
    OVERALL_STATUS="fail"
  fi

  TABLE_ROWS+=("| $name | $measured_allocs | $measured_bytes | $max_allocs | $max_bytes | $status |")
done < "$THRESHOLDS_FILE"

print_report() {
  echo "## Performance regression gate"
  echo ""
  echo "| Benchmark | allocs/op | B/op | max allocs/op | max B/op | status |"
  echo "|-----------|-----------|------|----------------|----------|--------|"
  printf '%s\n' "${TABLE_ROWS[@]}"
  echo ""
  if [[ "$OVERALL_STATUS" == "pass" ]]; then
    echo "All gated benchmarks are within their committed ceilings."
  else
    echo "**Regression detected:**"
    echo ""
    for f in "${FAILURES[@]}"; do
      echo "- $f"
    done
  fi
}

print_report

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  print_report >> "$GITHUB_STEP_SUMMARY"
fi

if [[ "$OVERALL_STATUS" != "pass" ]]; then
  exit 1
fi
