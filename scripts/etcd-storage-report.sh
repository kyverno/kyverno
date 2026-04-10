#!/usr/bin/env bash
# Query Prometheus for etcd storage metrics and write a JSON report.
# Usage: etcd-storage-report.sh [prometheus_url]
#   prometheus_url defaults to http://localhost:9090
# Writes etcd-storage-report.json in the current directory.

set -e

PROMETHEUS_URL="${1:-http://localhost:9090}"
OUTPUT_FILE="${ETCD_REPORT_OUTPUT:-etcd-storage-report.json}"

query() {
  local metric="$1"
  curl -sS --max-time 10 "${PROMETHEUS_URL}/api/v1/query?query=${metric}" 2>/dev/null | jq -r '
    .data.result[0].value[1] // empty
  ' 2>/dev/null || true
}

# Query etcd storage metrics (standard names from etcd /metrics)
total_size=$(query 'etcd_mvcc_db_total_size_in_bytes')
in_use=$(query 'etcd_mvcc_db_total_size_in_use_in_bytes')
quota=$(query 'etcd_server_quota_backend_bytes')
# Total keys in etcd (debugging metric; may be absent on some versions)
total_keys=$(query 'etcd_debugging_mvcc_keys_total')

# Report CR counts (stored in etcd) - optional if kubectl available
policy_report_count=""
cluster_policy_report_count=""
if command -v kubectl &>/dev/null; then
  policy_report_count=$(kubectl get policyreports -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
  cluster_policy_report_count=$(kubectl get clusterpolicyreports --no-headers 2>/dev/null | wc -l | tr -d ' ')
fi

if [[ -z "$total_size" && -z "$in_use" ]]; then
  echo '{"available":false,"message":"etcd storage metrics were not available"}' > "$OUTPUT_FILE"
  echo "Wrote $OUTPUT_FILE (no etcd metrics)"
  exit 0
fi

# Usage % of quota if quota is set
usage_pct=""
if [[ -n "$quota" && "$quota" -gt 0 && -n "$total_size" ]]; then
  usage_pct=$(awk "BEGIN {printf \"%.2f\", $total_size*100/$quota}" 2>/dev/null || true)
fi

# Build JSON with jq (numeric values; use null for missing)
jq -n \
  --arg t "${total_size:-}" \
  --arg i "${in_use:-}" \
  --arg q "${quota:-}" \
  --arg u "${usage_pct:-}" \
  --arg k "${total_keys:-}" \
  --arg pr "${policy_report_count:-}" \
  --arg cpr "${cluster_policy_report_count:-}" \
  '{
    available: true,
    db_total_size_bytes: (if $t == "" then null else ($t | tonumber) end),
    db_total_size_in_use_bytes: (if $i == "" then null else ($i | tonumber) end),
    quota_backend_bytes: (if $q == "" then null else ($q | tonumber) end),
    usage_quota_percent: (if $u == "" then null else ($u | tonumber) end),
    total_keys: (if $k == "" then null else ($k | tonumber) end),
    policy_report_count: (if $pr == "" then null else ($pr | tonumber) end),
    cluster_policy_report_count: (if $cpr == "" then null else ($cpr | tonumber) end)
  }' > "$OUTPUT_FILE" 2>/dev/null || {
  echo '{"available":false,"message":"failed to build report"}' > "$OUTPUT_FILE"
}

echo "Wrote $OUTPUT_FILE"
