#!/usr/bin/env bash
# Regenerate scripts/bench/thresholds.txt ceilings from a fresh benchmark
# run. This is the "ratchet" step: re-measure, then commit the tightened
# (or, for a legitimate increase, loosened) numbers in the same PR.
#
# Usage: bench-baseline.sh <bench-output-file> <thresholds-file>
#
# <bench-output-file> is the raw stdout of a `go test -bench=. -benchmem`
# run (ideally with -count=10, so the max across runs absorbs run-to-run
# jitter rather than committing a ceiling that a rerun immediately trips).
# <thresholds-file> is overwritten in place.
#
# For each gated benchmark, the ceiling is:
#   max_allocs_per_op = ceil(max(observed allocs/op) * (1 + ALLOCS_HEADROOM))
#   max_bytes_per_op  = ceil(max(observed bytes/op)  * (1 + BYTES_HEADROOM))
# Every currently gated benchmark is written synchronously with respect to
# its own async work (see each benchmark's doc comment for how), so a single
# headroom applies uniformly; if a future benchmark can't be made
# deterministic, give it a per-benchmark override here and document why.
#
# This script is a pure function of its input file: running it twice over
# the same bench-output-file produces a byte-identical thresholds-file. A
# second full measure-and-write can legitimately move a noisy benchmark's
# row - that is expected, not a bug in this script.
#
# Only benchmarks already gated in the existing <thresholds-file> are
# re-measured and rewritten - this script never adds a new gated row for a
# benchmark it happens to find in the bench output. Promoting an exploratory
# benchmark (for example a `BenchmarkHandle` added by an unrelated PR) into
# the gate is a deliberate decision made by hand-editing thresholds.txt, not
# a side effect of ratcheting the existing rows.
#
# The header this script writes is limited to stable, generic policy text.
# Anything specific to a moment in time (for example, "these numbers predate
# PR #12345") belongs as a hand-written comment in the committed
# thresholds-file, because this script overwrites the whole file and would
# otherwise reproduce a now-stale claim on every re-baseline.

set -euo pipefail

usage() {
  echo "Usage: $0 <bench-output-file> <thresholds-file>" >&2
  exit 1
}

BENCH_FILE="${1:?missing bench-output-file}"
THRESHOLDS_FILE="${2:?missing thresholds-file}"

[[ -f "$BENCH_FILE" ]] || { echo "Error: $BENCH_FILE not found" >&2; exit 1; }
[[ -f "$THRESHOLDS_FILE" ]] || { echo "Error: $THRESHOLDS_FILE not found (this script re-baselines an existing gated set, it does not create one)" >&2; exit 1; }

# The set of benchmark names already gated in the existing thresholds file,
# in file order. This script re-measures exactly this set and nothing else -
# see the "only benchmarks already gated" note above.
GATED_NAMES=$(mktemp)
trap 'rm -f "$GATED_NAMES"' EXIT
awk '{ line = $0; sub(/#.*/, "", line); gsub(/^[ \t]+|[ \t]+$/, "", line); if (line != "") print $1 }' "$THRESHOLDS_FILE" > "$GATED_NAMES"

[[ -s "$GATED_NAMES" ]] || { echo "Error: no gated benchmark rows found in $THRESHOLDS_FILE" >&2; exit 1; }

# Default headroom. Add a per-benchmark override here (mirroring this
# shape) only for a benchmark that genuinely cannot be made deterministic;
# document the reason in both this script and that benchmark's doc comment.
ALLOCS_HEADROOM_PCT=5
ALLOCS_HEADROOM_MIN=2
BYTES_HEADROOM_PCT=15

headroom_for() {
  local _name="$1" # unused while every gated benchmark uses default headroom
  echo "$ALLOCS_HEADROOM_PCT $BYTES_HEADROOM_PCT"
}

# Max allocs/op and bytes/op per benchmark name across all -count=N runs in
# the bench output.
MAXES=$(mktemp)
trap 'rm -f "$MAXES" "$GATED_NAMES"' EXIT

awk '
  /^Benchmark/ {
    name = $1
    sub(/-[0-9]+$/, "", name)
    allocs = $(NF-1) + 0
    bytes = $(NF-3) + 0
    if (!(name in maxAllocs) || allocs > maxAllocs[name]) { maxAllocs[name] = allocs }
    if (!(name in maxBytes) || bytes > maxBytes[name]) { maxBytes[name] = bytes }
    order[++n] = name
    seen[name] = 1
  }
  END {
    for (i = 1; i <= n; i++) {
      name = order[i]
      if (seen[name] == 1) {
        print name, maxAllocs[name], maxBytes[name]
        seen[name] = 2
      }
    }
  }
' "$BENCH_FILE" > "$MAXES"

[[ -s "$MAXES" ]] || { echo "Error: no Benchmark lines found in $BENCH_FILE" >&2; exit 1; }

# Fail before touching THRESHOLDS_FILE if any previously-gated benchmark is
# absent from this bench run (renamed, deleted, or the wrong package list
# was measured) - silently dropping its row would quietly ungate it.
MISSING=()
while IFS= read -r name; do
  [[ -z "$name" ]] && continue
  if ! awk -v n="$name" '$1 == n { found=1; exit } END { exit !found }' "$MAXES"; then
    MISSING+=("$name")
  fi
done < "$GATED_NAMES"

if (( ${#MISSING[@]} > 0 )); then
  echo "Error: gated benchmark(s) missing from $BENCH_FILE, thresholds file left unchanged:" >&2
  printf '  - %s\n' "${MISSING[@]}" >&2
  exit 1
fi

ceil_pct() {
  # ceil(value * (100 + pct) / 100), integer arithmetic only.
  local value="$1" pct="$2"
  echo $(( (value * (100 + pct) + 99) / 100 ))
}

TMP_OUT=$(mktemp)
trap 'rm -f "$MAXES" "$GATED_NAMES" "$TMP_OUT"' EXIT

{
  cat <<'HEADER'
# Allocation ceilings for the CEL admission-path benchmark gate (#17509).
#
# Format: <benchmark-name> <max_allocs_per_op> <max_bytes_per_op>
#
# This gate runs post-merge (see the `perf` job in
# .github/workflows/check-tests.yaml), not per pull request: a failure files
# or updates a workflow-failure issue on main rather than blocking anyone's
# PR, which is what makes it safe to keep this headroom tight (see
# docs/dev/README.md, "Performance benchmarks").
#
# Values below are measured on linux/amd64 (the CI platform - allocation
# counts differ across GOOS/GOARCH, so darwin/arm64 numbers must never be
# committed here) with:
#   go test -run=^$ -bench=. -benchmem -benchtime=100x -count=10 \
#     ./pkg/cel/policies/vpol/engine ./pkg/cel/policies/mpol/engine \
#     ./pkg/webhooks/resource/vpol ./pkg/webhooks/resource/mpol
# inside a `golang` Docker container, taking the max observed allocs/op and
# bytes/op across the 10 runs per benchmark.
#
# Headroom: +5% on allocs/op (minimum +2), +15% on bytes/op, rounded up. Each
# gated benchmark synchronizes its own async (unwaited-goroutine) work before
# the timed loop advances - see each benchmark's doc comment for how - so a
# single headroom applies uniformly; a future benchmark that genuinely can't
# be made deterministic should get a documented override instead.
#
# Ratchet rule: if your PR legitimately changes allocation counts at one of
# these boundaries (an intentional improvement or a justified increase), run
# `make bench-baseline` and commit the regenerated ceilings in the same PR,
# with a one-line justification in the PR description. At each release
# branch cut, the release owner re-runs `make bench-baseline` on linux/amd64
# as a periodic backstop against ceiling staleness.

# name                                    max_allocs_per_op  max_bytes_per_op
HEADER

  while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    read -r _ allocs bytes <<< "$(awk -v n="$name" '$1 == n { print; exit }' "$MAXES")"
    read -r allocs_pct bytes_pct <<< "$(headroom_for "$name")"
    max_allocs=$(ceil_pct "$allocs" "$allocs_pct")
    min_allocs=$(( allocs + ALLOCS_HEADROOM_MIN ))
    if (( min_allocs > max_allocs )); then
      max_allocs=$min_allocs
    fi
    max_bytes=$(ceil_pct "$bytes" "$bytes_pct")
    printf '%-40s %-18s %s\n' "$name" "$max_allocs" "$max_bytes"
  done < "$GATED_NAMES"
} > "$TMP_OUT"

mv "$TMP_OUT" "$THRESHOLDS_FILE"
echo "Wrote $(grep -c '^Benchmark' "$THRESHOLDS_FILE") ceiling(s) to $THRESHOLDS_FILE" >&2
