#!/usr/bin/env bash
set -euo pipefail
phase=$1
namespace=$2
controller_namespace=${KYVERNO_NAMESPACE:-kyverno}
expected=$(mktemp)
actual=$(mktemp)
trap 'rm -f "$expected" "$actual"' EXIT
printf '%s\n' 'policies.kyverno.io|retained-inventory|ValidatingPolicy|retained-policy|1' > "$expected"
case "$phase" in
  created)
    printf '%s\n' \
      'kyverno.io|legacy-inventory|ClusterPolicy|missing-cluster-policy|1' \
      'kyverno.io|legacy-inventory|Policy|other/missing-policy|1' \
      'policies.kyverno.io|cel-inventory|ValidatingPolicy|missing-cel-policy|1' >> "$expected" ;;
  updated)
    printf '%s\n' \
      'kyverno.io|legacy-inventory|ClusterPolicy|replacement-legacy|1' \
      'policies.kyverno.io|cel-inventory|ValidatingPolicy|replacement-cel|1' >> "$expected" ;;
  deleted) ;;
  *) exit 2 ;;
esac
sort -o "$expected" "$expected"
for attempt in $(seq 1 60); do
  success=true
  pods=$(kubectl -n "$controller_namespace" get pods -l app.kubernetes.io/component=admission-controller -o jsonpath='{.items[*].metadata.name}')
  if [[ -z "$pods" ]]; then success=false; fi
  for pod in $pods; do
    if ! body=$(kubectl get --raw "/api/v1/namespaces/$controller_namespace/pods/$pod:8000/proxy/metrics"); then
      success=false
      continue
    fi
    if ! grep -q '^# TYPE kyverno_policy_exception_info gauge$' <<< "$body"; then success=false; fi
    awk -v ns="$namespace" '
      /^kyverno_policy_exception_info\{/ {
        delete labels
        split($0, parts, /[{}]/)
        n = split(parts[2], fields, ",")
        for (i=1; i<=n; i++) {
          split(fields[i], kv, "=")
          gsub(/"/, "", kv[2])
          labels[kv[1]]=kv[2]
        }
        if (labels["exception_namespace"] == ns) {
          value=parts[3]; gsub(/^ +| +$/, "", value)
          print labels["exception_api_group"] "|" labels["exception_name"] "|" labels["policy_kind"] "|" labels["policy_name"] "|" value
        }
      }' <<< "$body" | sort > "$actual"
    if ! diff -u "$expected" "$actual"; then success=false; fi
  done
  if "$success"; then printf 'Verified %s on all admission replicas\n' "$phase"; exit 0; fi
  sleep 1
done
exit 1
