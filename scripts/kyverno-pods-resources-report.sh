#!/usr/bin/env bash
# Query Prometheus for CPU and memory usage of pods in the kyverno namespace,
# aggregated by app.kubernetes.io/name label. Outputs avg, min, med, max, p90, p95, p99.
# Usage: kyverno-pods-resources-report.sh [prometheus_url]
# Writes kyverno-pods-resources-report.json in the current directory.

set -e

PROMETHEUS_URL="${1:-http://localhost:9090}"
OUTPUT_FILE="${KYVERNO_PODS_REPORT_OUTPUT:-kyverno-pods-resources-report.json}"
NAMESPACE="${KYVERNO_NAMESPACE:-kyverno}"
# Time window for range query (seconds) and step (seconds)
RANGE_SEC="${KYVERNO_PODS_RANGE:-300}"
STEP_SEC="${KYVERNO_PODS_STEP:-15}"

# Pod -> app.kubernetes.io/name mapping (fallback to pod name if label missing or kubectl unavailable)
pod_to_app='[]'
if command -v kubectl &>/dev/null; then
  pod_to_app=$(kubectl get pods -n "$NAMESPACE" -o json 2>/dev/null | jq -c '
    [.items[]? | {pod: .metadata.name, app: (.metadata.labels["app.kubernetes.io/name"] // .metadata.name)}]
  ' 2>/dev/null) || pod_to_app='[]'
fi
[[ -z "$pod_to_app" ]] && pod_to_app='[]'

# Prometheus query_range: returns .data.result[].values = [[ts, val], ...]
query_range() {
  local query="$1"
  local end_sec start_sec
  end_sec=$(date +%s)
  start_sec=$((end_sec - RANGE_SEC))
  curl -sS --max-time 30 "${PROMETHEUS_URL}/api/v1/query_range" \
    --data-urlencode "query=${query}" \
    --data-urlencode "start=${start_sec}" \
    --data-urlencode "end=${end_sec}" \
    --data-urlencode "step=${STEP_SEC}s" \
    2>/dev/null | jq -c '.data.result // []' 2>/dev/null || echo '[]'
}

# cAdvisor/kubelet metrics; exclude POD and empty container
MEM_QUERY="sum(container_memory_working_set_bytes{namespace=\"${NAMESPACE}\", container!=\"\", container!=\"POD\"}) by (pod)"
CPU_QUERY="sum(rate(container_cpu_usage_seconds_total{namespace=\"${NAMESPACE}\", container!=\"\", container!=\"POD\"}[1m])) by (pod)"

mem_results=$(query_range "$MEM_QUERY")
cpu_results=$(query_range "$CPU_QUERY")
[[ -z "$mem_results" ]] && mem_results="[]"
[[ -z "$cpu_results" ]] && cpu_results="[]"

# Build per-pod series from range results
mem_pods_json=$(echo "$mem_results" | jq -c '
  map({ pod: .metric.pod, values: (.values | map(.[1] | tonumber)) })
')
cpu_pods_json=$(echo "$cpu_results" | jq -c '
  map({ pod: .metric.pod, values: (.values | map(.[1] | tonumber)) })
')

# Aggregate by app (app.kubernetes.io/name); when pod_to_app is empty, use pod name as app
merged=$(jq -n \
  --argjson mem "$mem_pods_json" \
  --argjson cpu "$cpu_pods_json" \
  --argjson pod_to_app "$pod_to_app" \
  '
    def stats(v):
      (v | sort) as $s |
      if ($s | length) == 0 then {avg: null, min: null, med: null, max: null, p90: null, p95: null, p99: null}
      else
        ($s | add / length) as $avg |
        ($s[(($s | length) * 50 / 100) - 1] // $s[0]) as $med |
        ($s[(($s | length) * 90 / 100) - 1] // $s[-1]) as $p90 |
        ($s[(($s | length) * 95 / 100) - 1] // $s[-1]) as $p95 |
        ($s[(($s | length) * 99 / 100) - 1] // $s[-1]) as $p99 |
        {
          avg: (if $avg then ($avg * 1000 | floor / 1000) else null end),
          min: (if $s[0] != null then ($s[0] * 1000 | floor / 1000) else null end),
          med: (if $med != null then ($med * 1000 | floor / 1000) else null end),
          max: (if $s[-1] != null then ($s[-1] * 1000 | floor / 1000) else null end),
          p90: (if $p90 != null then ($p90 * 1000 | floor / 1000) else null end),
          p95: (if $p95 != null then ($p95 * 1000 | floor / 1000) else null end),
          p99: (if $p99 != null then ($p99 * 1000 | floor / 1000) else null end)
        }
      end;

    # pod -> app mapping; if empty, identity (pod name = app name)
    ($pod_to_app | if length == 0 then (($mem | map(.pod)) + ($cpu | map(.pod)) | unique | map({pod: ., app: .})) else . end) as $mapping |
    # group by app: list of {app, pods: [pod names]}
    ($mapping | group_by(.app) | map({app: .[0].app, pods: map(.pod)})) as $app_groups |

    ($app_groups | map(
      .app as $app |
      .pods as $pods |
      (($mem | map(select(.pod as $p | $pods | index($p) != null)) | [.[].values] | transpose | map(add)) // []) as $mem_series |
      (($cpu | map(select(.pod as $p | $pods | index($p) != null)) | [.[].values] | transpose | map(add)) // []) as $cpu_series |
      { app: $app, memory_bytes: ($mem_series | stats(.)), cpu_cores: ($cpu_series | stats(.)) }
    )) as $apps |

    (([$mem[].values] | transpose | map(add)) // []) as $mem_series |
    (([$cpu[].values] | transpose | map(add)) // []) as $cpu_series |

    {
      available: true,
      namespace: "kyverno",
      apps: $apps,
      totals: {
        memory_bytes: ($mem_series | stats(.)),
        cpu_cores: ($cpu_series | stats(.))
      }
    }
  ' 2>/dev/null) || merged=""

if [[ -z "$merged" || "$merged" == "null" ]]; then
  echo '{"available":false,"namespace":"kyverno","apps":[],"totals":{"memory_bytes":null,"cpu_cores":null}}' > "$OUTPUT_FILE"
else
  echo "$merged" > "$OUTPUT_FILE"
fi

echo "Wrote $OUTPUT_FILE"
