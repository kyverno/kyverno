#!/usr/bin/env bash
# Aggregate etcd-storage-report.json from multiple jobs into a single comparison table.
# Usage: etcd-aggregate-summary.sh <base-dir> [job1] [job2] ...
#   base-dir: directory containing one subdir per job, each with etcd-storage-report.json
#   job names: optional list; if omitted, all subdirs of base-dir are used

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
    awk "BEGIN {printf \"%.2f MB\", $v/1048576}" 2>/dev/null || echo "—"
  fi
}

pct() {
  local v="$1"
  if [[ -z "$v" || "$v" == "null" ]]; then
    echo "—"
  else
    awk "BEGIN {printf \"%.1f%%\", $v}" 2>/dev/null || echo "—"
  fi
}

ROWS=()
for job in "${JOBS[@]}"; do
  f="$BASE_DIR/$job/etcd-storage-report.json"
  if [[ ! -f "$f" ]]; then
    continue
  fi
  available=$(jq -r '.available // false' "$f" 2>/dev/null)
  if [[ "$available" != "true" ]]; then
    ROWS+=("| $job | — | — | — | — | — | — | — |")
    continue
  fi
  total=$(jq -r '.db_total_size_bytes // "null"' "$f")
  in_use=$(jq -r '.db_total_size_in_use_bytes // "null"' "$f")
  quota=$(jq -r '.quota_backend_bytes // "null"' "$f")
  usage=$(jq -r '.usage_quota_percent // "null"' "$f")
  keys=$(jq -r '.total_keys // "null"' "$f")
  pr_count=$(jq -r '.policy_report_count // "null"' "$f")
  cpr_count=$(jq -r '.cluster_policy_report_count // "null"' "$f")
  keys_fmt="${keys}"
  [[ -z "$keys" || "$keys" == "null" ]] && keys_fmt="—"
  pr_fmt="${pr_count}"
  [[ -z "$pr_count" || "$pr_count" == "null" ]] && pr_fmt="—"
  cpr_fmt="${cpr_count}"
  [[ -z "$cpr_count" || "$cpr_count" == "null" ]] && cpr_fmt="—"
  ROWS+=("| $job | $(bytes_to_mb "$total") | $(bytes_to_mb "$in_use") | $(bytes_to_mb "$quota") | $(pct "$usage") | $keys_fmt | $pr_fmt | $cpr_fmt |")
done

if [[ ${#ROWS[@]} -eq 0 ]]; then
  echo "## etcd storage summary"
  echo ""
  echo "No etcd reports found under \`$BASE_DIR\`."
  exit 0
fi

echo "## etcd storage summary"
echo ""
echo "| Job | DB total | In use | Quota | Usage | Keys | PolicyReports | ClusterPolicyReports |"
echo "|-----|---------|--------|-------|-------|------|---------------|----------------------|"
printf '%s\n' "${ROWS[@]}"
echo ""
