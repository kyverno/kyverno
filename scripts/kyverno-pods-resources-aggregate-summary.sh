#!/usr/bin/env bash
# Aggregate kyverno-pods-resources-report.json from multiple jobs.
# Report JSON has apps[] (by app.kubernetes.io/name) or pods[] with {memory_bytes,cpu_cores} and totals.
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

# Check if report has new stats shape (totals.memory_bytes.avg) vs old (totals.memory_bytes as number)
has_distribution() {
  jq -e '.totals.memory_bytes | type == "object"' "$1" >/dev/null 2>/dev/null
}

echo "## Kyverno namespace – CPU & memory (by app)"
echo ""

# Comparison table across scenarios (totals only)
echo "### Comparison across scenarios"
echo ""
echo "| Scenario | CPU avg | CPU med | CPU p99 | Mem avg | Mem med | Mem p99 |"
echo "|----------|---------|---------|---------|---------|---------|---------|"

for job in "${JOBS[@]}"; do
  f="$BASE_DIR/$job/kyverno-pods-resources-report.json"
  if [[ ! -f "$f" ]]; then
    echo "| $job | — | — | — | — | — | — |"
    continue
  fi
  available=$(jq -r '.available // false' "$f" 2>/dev/null)
  if [[ "$available" != "true" ]]; then
    echo "| $job | — | — | — | — | — | — |"
    continue
  fi
  if has_distribution "$f"; then
    jq -r --arg job "$job" '
      . as $r |
      ($r.totals.cpu_cores.avg | if . != null then (. * 1000 | floor / 1000 | tostring) else "—" end) as $ca |
      ($r.totals.cpu_cores.med | if . != null then (. * 1000 | floor / 1000 | tostring) else "—" end) as $cm |
      ($r.totals.cpu_cores.p99 | if . != null then (. * 1000 | floor / 1000 | tostring) else "—" end) as $c99 |
      ($r.totals.memory_bytes.avg | if . != null then ((. / 1048576) * 10 | floor / 10 | tostring) + " MB" else "—" end) as $ma |
      ($r.totals.memory_bytes.med | if . != null then ((. / 1048576) * 10 | floor / 10 | tostring) + " MB" else "—" end) as $mm |
      ($r.totals.memory_bytes.p99 | if . != null then ((. / 1048576) * 10 | floor / 10 | tostring) + " MB" else "—" end) as $m99 |
      "| " + $job + " | " + $ca + " | " + $cm + " | " + $c99 + " | " + $ma + " | " + $mm + " | " + $m99 + " |"
    ' "$f" 2>/dev/null
  else
    total_cpu=$(jq -r '.totals.cpu_cores // "null"' "$f")
    total_mem=$(jq -r '.totals.memory_bytes // "null"' "$f")
    echo "| $job | $(cpu_fmt "$total_cpu") | $(cpu_fmt "$total_cpu") | $(cpu_fmt "$total_cpu") | $(bytes_to_mb "$total_mem") | $(bytes_to_mb "$total_mem") | $(bytes_to_mb "$total_mem") |"
  fi
done
echo ""

# Matrix: apps (rows) × scenarios (columns)
ALL_APPS=$(for job in "${JOBS[@]}"; do
  f="$BASE_DIR/$job/kyverno-pods-resources-report.json"
  [[ -f "$f" ]] && jq -r '(.apps[]? // .pods[]?) | (.app // .pod)' "$f" 2>/dev/null
done | sort -u)
if [[ -n "$ALL_APPS" ]]; then
  echo "### By app × scenario (CPU avg, Mem avg)"
  echo ""
  # Header: App | scenario1 | scenario2 | ...
  header="| App |"
  for job in "${JOBS[@]}"; do header="$header $job |"; done
  echo "$header"
  sep="|-----|"
  for _ in "${JOBS[@]}"; do sep="$sep---------|"; done
  echo "$sep"
  while IFS= read -r app; do
    [[ -z "$app" ]] && continue
    row="| $app |"
    for job in "${JOBS[@]}"; do
      f="$BASE_DIR/$job/kyverno-pods-resources-report.json"
      cell="—"
      if [[ -f "$f" ]] && [[ "$(jq -r '.available // false' "$f" 2>/dev/null)" == "true" ]]; then
        if has_distribution "$f"; then
          entry=$(jq -r --arg app "$app" '
            (.apps[]? // .pods[]?) | select((.app // .pod) == $app) |
            (if .cpu_cores.avg != null then ((.cpu_cores.avg * 1000 | floor) / 1000 | tostring) + " cores" else "" end) as $c |
            (if .memory_bytes.avg != null then ((.memory_bytes.avg / 1048576) * 10 | floor / 10 | tostring) + " MB" else "" end) as $m |
            (if $c != "" and $m != "" then $c + ", " + $m elif $c != "" then $c elif $m != "" then $m else "—" end)
          ' "$f" 2>/dev/null)
          [[ -n "$entry" ]] && cell="$entry"
        else
          entry=$(jq -r --arg app "$app" '
            (.apps[]? // .pods[]?) | select((.app // .pod) == $app) |
            (if .cpu_cores != null then ((.cpu_cores * 1000 | floor) / 1000 | tostring) + " cores" else "" end) as $c |
            (if .memory_bytes != null then ((.memory_bytes / 1048576) * 10 | floor / 10 | tostring) + " MB" else "" end) as $m |
            (if $c != "" and $m != "" then $c + ", " + $m elif $c != "" then $c elif $m != "" then $m else "—" end)
          ' "$f" 2>/dev/null)
          [[ -n "$entry" ]] && cell="$entry"
        fi
      fi
      row="$row $cell |"
    done
    echo "$row"
  done <<< "$ALL_APPS"
  echo ""
fi

# Per-scenario details
echo "### Details by scenario"
echo ""
for job in "${JOBS[@]}"; do
  f="$BASE_DIR/$job/kyverno-pods-resources-report.json"
  if [[ ! -f "$f" ]]; then
    echo "**$job** — no report file"
    echo ""
    continue
  fi
  available=$(jq -r '.available // false' "$f" 2>/dev/null)
  if [[ "$available" != "true" ]]; then
    echo "**$job** — no data"
    echo ""
    continue
  fi

  if has_distribution "$f"; then
    # New format: totals.memory_bytes.{avg,min,med,max,p90,p95,p99}
    echo "### $job – totals (avg, min, med, max, p90, p95, p99)"
    echo ""
    echo "#### CPU (cores)"
    echo ""
    echo "| avg | min | med | max | p90 | p95 | p99 |"
    echo "|-----|-----|-----|-----|-----|-----|-----|"
    jq -r '
      .totals.cpu_cores |
      "| " + (if .avg != null then (.avg | tostring) else "—" end) +
      " | " + (if .min != null then (.min | tostring) else "—" end) +
      " | " + (if .med != null then (.med | tostring) else "—" end) +
      " | " + (if .max != null then (.max | tostring) else "—" end) +
      " | " + (if .p90 != null then (.p90 | tostring) else "—" end) +
      " | " + (if .p95 != null then (.p95 | tostring) else "—" end) +
      " | " + (if .p99 != null then (.p99 | tostring) else "—" end) + " |"
    ' "$f" 2>/dev/null
    echo ""
    echo "#### Memory"
    echo ""
    echo "| avg | min | med | max | p90 | p95 | p99 |"
    echo "|-----|-----|-----|-----|-----|-----|-----|"
    jq -r '
      .totals.memory_bytes |
      "| " + (if .avg != null then ((.avg / 1048576) | . * 10 | floor / 10 | tostring) + " MB" else "—" end) +
      " | " + (if .min != null then ((.min / 1048576) | . * 10 | floor / 10 | tostring) + " MB" else "—" end) +
      " | " + (if .med != null then ((.med / 1048576) | . * 10 | floor / 10 | tostring) + " MB" else "—" end) +
      " | " + (if .max != null then ((.max / 1048576) | . * 10 | floor / 10 | tostring) + " MB" else "—" end) +
      " | " + (if .p90 != null then ((.p90 / 1048576) | . * 10 | floor / 10 | tostring) + " MB" else "—" end) +
      " | " + (if .p95 != null then ((.p95 / 1048576) | . * 10 | floor / 10 | tostring) + " MB" else "—" end) +
      " | " + (if .p99 != null then ((.p99 / 1048576) | . * 10 | floor / 10 | tostring) + " MB" else "—" end) + " |"
    ' "$f" 2>/dev/null
    echo ""
    echo "#### Per app (avg, med, p99)"
    echo ""
    echo "| App | CPU avg | CPU med | CPU p99 | Mem avg | Mem med | Mem p99 |"
    echo "|-----|---------|---------|---------|---------|---------|---------|"
    jq -r '
      (.apps[]? // .pods[]?) |
      (.app // .pod) as $name |
      (if .cpu_cores.avg != null then ((.cpu_cores.avg * 1000 | floor) / 1000 | tostring) else "—" end) as $ca |
      (if .cpu_cores.med != null then ((.cpu_cores.med * 1000 | floor) / 1000 | tostring) else "—" end) as $cm |
      (if .cpu_cores.p99 != null then ((.cpu_cores.p99 * 1000 | floor) / 1000 | tostring) else "—" end) as $c99 |
      (if .memory_bytes.avg != null then ((.memory_bytes.avg / 1048576) * 10 | floor / 10 | tostring) + " MB" else "—" end) as $ma |
      (if .memory_bytes.med != null then ((.memory_bytes.med / 1048576) * 10 | floor / 10 | tostring) + " MB" else "—" end) as $mm |
      (if .memory_bytes.p99 != null then ((.memory_bytes.p99 / 1048576) * 10 | floor / 10 | tostring) + " MB" else "—" end) as $m99 |
      "| " + $name + " | " + $ca + " | " + $cm + " | " + $c99 + " | " + $ma + " | " + $mm + " | " + $m99 + " |"
    ' "$f" 2>/dev/null || true
  else
    # Legacy format: totals.memory_bytes and totals.cpu_cores as single numbers
    total_mem=$(jq -r '.totals.memory_bytes // "null"' "$f")
    total_cpu=$(jq -r '.totals.cpu_cores // "null"' "$f")
    echo "### $job"
    echo ""
    echo "| Total CPU (cores) | Total memory |"
    echo "|-------------------|--------------|"
    echo "| $(cpu_fmt "$total_cpu") | $(bytes_to_mb "$total_mem") |"
    echo ""
    echo "| App | CPU (cores) | Memory |"
    echo "|-----|-------------|--------|"
    jq -r '
      (.apps[]? // .pods[]?) |
      "| " + (.app // .pod) + " | " + ((.cpu_cores | . * 1000 | floor / 1000 | tostring) // "—") + " | " + ((.memory_bytes / 1048576 | . * 10 | floor / 10 | tostring) + " MB") + " |"
    ' "$f" 2>/dev/null || true
  fi
  echo ""
done
