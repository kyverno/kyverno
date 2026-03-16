#!/usr/bin/env bash
# Query Prometheus for CPU and memory usage of all pods in the kyverno namespace.
# Usage: kyverno-pods-resources-report.sh [prometheus_url]
# Writes kyverno-pods-resources-report.json in the current directory.

set -e

PROMETHEUS_URL="${1:-http://localhost:9090}"
OUTPUT_FILE="${KYVERNO_PODS_REPORT_OUTPUT:-kyverno-pods-resources-report.json}"
NAMESPACE="${KYVERNO_NAMESPACE:-kyverno}"

# Query Prometheus; returns JSON with .data.result[] (each has .metric.pod, .value[1])
query_vector() {
  local query="$1"
  curl -sS --max-time 15 "${PROMETHEUS_URL}/api/v1/query" --data-urlencode "query=${query}" 2>/dev/null | jq -c '.data.result // []' 2>/dev/null || echo '[]'
}

# cAdvisor/kubelet metrics (container_*); exclude POD and empty container
MEM_QUERY="sum(container_memory_working_set_bytes{namespace=\"${NAMESPACE}\", container!=\"\", container!=\"POD\"}) by (pod)"
CPU_QUERY="sum(rate(container_cpu_usage_seconds_total{namespace=\"${NAMESPACE}\", container!=\"\", container!=\"POD\"}[1m])) by (pod)"

mem_results=$(query_vector "$MEM_QUERY")
cpu_results=$(query_vector "$CPU_QUERY")
[[ -z "$mem_results" ]] && mem_results="[]"
[[ -z "$cpu_results" ]] && cpu_results="[]"

# Build merged array: [{pod, memory_bytes, cpu_cores}, ...]
# If no pods in kyverno namespace, results are empty and we still write valid JSON.
merged=$(jq -n \
  --argjson mem "$mem_results" \
  --argjson cpu "$cpu_results" \
  '
    (if $mem | type == "array" then $mem else [] end | map({pod: .metric.pod, memory_bytes: (.value[1] | tonumber)})) as $mem_by_pod |
    (if $cpu | type == "array" then $cpu else [] end | map({pod: .metric.pod, cpu_cores: (.value[1] | tonumber)})) as $cpu_by_pod |
    ($mem_by_pod | map(.pod)) + ($cpu_by_pod | map(.pod)) | unique | map(
      . as $p |
      (($mem_by_pod | map(select(.pod == $p)) | .[0].memory_bytes) // 0) as $mb |
      (($cpu_by_pod | map(select(.pod == $p)) | .[0].cpu_cores) // 0) as $cc |
      {pod: $p, memory_bytes: $mb, cpu_cores: $cc}
    )
  ' 2>/dev/null) || merged="[]"

# Compute totals and build final report (add defaults to null for empty arrays)
jq -n \
  --argjson pods "${merged:-[]}" \
  --arg ns "$NAMESPACE" \
  '{
    available: true,
    namespace: $ns,
    pods: $pods,
    totals: {
      memory_bytes: (($pods | map(.memory_bytes) | add) // 0),
      cpu_cores: (($pods | map(.cpu_cores) | add) // 0)
    }
  }' > "$OUTPUT_FILE" 2>/dev/null || {
  echo '{"available":false,"namespace":"kyverno","pods":[],"totals":{"memory_bytes":null,"cpu_cores":null}}' > "$OUTPUT_FILE"
}

echo "Wrote $OUTPUT_FILE"
