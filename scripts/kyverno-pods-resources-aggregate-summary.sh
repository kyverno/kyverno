#!/usr/bin/env bash
# Aggregate kyverno-pods-resources-report.json from multiple jobs into a comparison table.
# Usage: kyverno-pods-resources-aggregate-summary.sh <base-dir> [job1] [job2] ...

set -e

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

bytes_to_mb() {
  local v="$1"
  if [[ -z "$v" || "$v" == "null" ]]; then
    echo "—"
  else
    awk "BEGIN {printf \"%.1f MB\", $v/1048576}" 2>/dev/null || echo "—"
  fi
}

cpu_fmt() {
  local v="$1"
  if [[ -z "$v" || "$v" == "null" ]]; then
    echo "—"
  else
    awk "BEGIN {printf \"%.3f\", $v}" 2>/dev/null || echo "—"
  fi
}

# Summary table: one row per job with totals
echo "## Kyverno namespace – CPU & memory (pods)"
echo ""
echo "| Job | Total CPU (cores) | Total memory |"
echo "|-----|-------------------|--------------|"

for job in "${JOBS[@]}"; do
  f="$BASE_DIR/$job/kyverno-pods-resources-report.json"
  if [[ ! -f "$f" ]]; then
    echo "| $job | — | — |"
    continue
  fi
  available=$(jq -r '.available // false' "$f" 2>/dev/null)
  if [[ "$available" != "true" ]]; then
    echo "| $job | — | — |"
    continue
  fi
  total_mem=$(jq -r '.totals.memory_bytes // "null"' "$f")
  total_cpu=$(jq -r '.totals.cpu_cores // "null"' "$f")
  echo "| $job | $(cpu_fmt "$total_cpu") | $(bytes_to_mb "$total_mem") |"
done
echo ""

# Per-job per-pod breakdown (collapsible-style: subsection per job)
for job in "${JOBS[@]}"; do
  f="$BASE_DIR/$job/kyverno-pods-resources-report.json"
  [[ -f "$f" ]] || continue
  [[ "$(jq -r '.available // false' "$f" 2>/dev/null)" == "true" ]] || continue
  pod_count=$(jq -r '.pods | length' "$f" 2>/dev/null)
  [[ -n "$pod_count" && "$pod_count" -gt 0 ]] || continue
  echo "### $job – per pod"
  echo ""
  echo "| Pod | CPU (cores) | Memory |"
  echo "|-----|-------------|--------|"
  jq -r '
    .pods[] |
    "| " + .pod + " | " + ((.cpu_cores | . * 1000 | floor / 1000) | tostring) + " | " + ((.memory_bytes / 1048576 | . * 10 | floor / 10) | tostring) + " MB |"
  ' "$f" 2>/dev/null || true
  echo ""
done
