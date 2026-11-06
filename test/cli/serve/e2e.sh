#!/usr/bin/env bash
# End-to-end test for the experimental `kyverno serve` command: starts one server per mode on a
# free loopback port and checks every fixture payload with curl.
#
# Usage: test/cli/serve/e2e.sh <path-to-kyverno-cli-binary>
# Requires: curl, jq.
set -euo pipefail

CLI="${1:?usage: $0 <path-to-kyverno-cli-binary>}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT_BASE="${KYVERNO_SERVE_E2E_PORT:-18480}"
FAILURES=0
PIDS=()
LOGS="$(mktemp -d)"

cleanup() {
  for pid in "${PIDS[@]:-}"; do
    [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
  done
  rm -rf "$LOGS"
}
trap cleanup EXIT

start_server() { # mode port policy-dir...
  local mode="$1" port="$2"
  shift 2
  local args=()
  for p in "$@"; do args+=(--policies "$p"); done
  KYVERNO_EXPERIMENTAL=true "$CLI" serve "${args[@]}" --mode "$mode" --address "127.0.0.1:$port" >"$LOGS/$mode.log" 2>&1 &
  PIDS+=("$!")
  for _ in $(seq 1 100); do
    if curl -sf "http://127.0.0.1:$port/readyz" >/dev/null; then
      return 0
    fi
    sleep 0.1
  done
  echo "server for $mode did not become ready:" >&2
  cat "$LOGS/$mode.log" >&2
  exit 1
}

check() { # name port mode payload expected-http-code jq-filter-that-must-be-true
  local name="$1" port="$2" mode="$3" payload="$4" want_code="$5" filter="$6"
  local body code
  body="$(mktemp)"
  code="$(curl -s -o "$body" -w '%{http_code}' -X POST --data-binary "@$payload" "http://127.0.0.1:$port/v1alpha1/evaluate/$mode")"
  if [ "$code" = "$want_code" ] && jq -e "$filter" "$body" >/dev/null; then
    echo "PASS  $name"
  else
    echo "FAIL  $name (HTTP $code, want $want_code; filter: $filter)"
    jq . "$body" 2>/dev/null || cat "$body"
    FAILURES=$((FAILURES + 1))
  fi
  rm -f "$body"
}

K=$((PORT_BASE)); J=$((PORT_BASE + 1)); H=$((PORT_BASE + 2)); E=$((PORT_BASE + 3))
start_server Kubernetes "$K" "$DIR/policies/kubernetes"
start_server JSON "$J" "$DIR/policies/json"
start_server HTTP "$H" "$DIR/policies/http"
start_server Envoy "$E" "$DIR/policies/envoy"

P="$DIR/payloads"
# Kubernetes
check "kubernetes: compliant deployment is allowed" "$K" Kubernetes "$P/kubernetes/deployment-pass.yaml" 200 \
  '.decision == "Allow" and .summary.fail == 0 and .apiVersion == "serve.cli.kyverno.io/v1alpha1"'
check "kubernetes: non-compliant deployment is denied" "$K" Kubernetes "$P/kubernetes/deployment-fail.yaml" 200 \
  '.decision == "Deny" and ([.results[] | select(.status == "fail") | .policy.name] == ["check-deployment-labels"])'
check "kubernetes: pod is mutated" "$K" Kubernetes "$P/kubernetes/pod.yaml" 200 \
  '.decision == "Allow" and .artifacts.patched[0].spec.containers[0].image == "registry.example.com/nginx:1.27"'
check "kubernetes: namespace generates a configmap" "$K" Kubernetes "$P/kubernetes/namespace.yaml" 200 \
  '.artifacts.generated[0].kind == "ConfigMap" and .artifacts.generated[0].metadata.namespace == "team-a"'
check "kubernetes: unmatched resource is NoMatch, not Allow" "$K" Kubernetes "$P/kubernetes/configmap.yaml" 200 \
  '.decision == "NoMatch" and (.results | length) == 0'
# JSON
check "json: compliant payload is allowed" "$J" JSON "$P/json/dockerfile-pass.json" 200 '.decision == "Allow"'
check "json: non-compliant payload is denied" "$J" JSON "$P/json/dockerfile-fail.json" 200 \
  '.decision == "Deny" and .results[0].message == "wget is not allowed"'
# HTTP
check "http: GET is allowed" "$H" HTTP "$P/http/get.json" 200 '.decision == "Allow"'
check "http: DELETE is denied" "$H" HTTP "$P/http/delete.json" 200 '.decision == "Deny" and .summary.fail == 1'
# Envoy
check "envoy: public path is allowed" "$E" Envoy "$P/envoy/public.json" 200 '.decision == "Allow"'
check "envoy: admin path is denied" "$E" Envoy "$P/envoy/admin.json" 200 '.decision == "Deny" and (.results[0].message | contains("403"))'
# Failures are explicit
check "malformed request is a 400" "$K" Kubernetes "$P/json/dockerfile-pass.json" 400 '.kind == "Status" and .status == "Failure"'
check "disabled mode is a 404" "$K" Envoy "$P/envoy/public.json" 404 '.message | contains("not enabled")'

# policies endpoint and digest stability
DIGEST="$(curl -sf "http://127.0.0.1:$K/v1alpha1/policies" | jq -r .policySetDigest)"
if [[ "$DIGEST" =~ ^sha256:[0-9a-f]{64}$ ]] && [ "$(curl -sf "http://127.0.0.1:$K/v1alpha1/policies" | jq '.policies | length')" = "3" ]; then
  echo "PASS  policies: lists 3 policies with digest $DIGEST"
else
  echo "FAIL  policies endpoint"
  FAILURES=$((FAILURES + 1))
fi

# invalid policies must fail startup
if KYVERNO_EXPERIMENTAL=true "$CLI" serve --policies "$DIR/policies/invalid" --address "127.0.0.1:$((PORT_BASE + 9))" >/dev/null 2>&1; then
  echo "FAIL  invalid policies did not fail startup"
  FAILURES=$((FAILURES + 1))
else
  echo "PASS  invalid policies fail startup"
fi

if [ "$FAILURES" -gt 0 ]; then
  echo "$FAILURES check(s) failed" >&2
  exit 1
fi
echo "all kyverno serve e2e checks passed"
