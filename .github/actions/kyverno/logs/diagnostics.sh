#!/usr/bin/env bash
# Collects the state of a failed E2E cluster into a directory, for upload as a
# diagnostics artifact: logs from all Kyverno pods, events, webhook
# configurations, policies with their status, and reports.
#
# Usage: diagnostics.sh <output-directory>
#
# Every command is best effort, because the cluster may be broken or missing.
# The output stays bounded: each log keeps its last LOG_LINES lines, and each
# file is cut at MAX_FILE_BYTES. Each kubectl call has a deadline, so a
# stalled call can't hold up the upload step that follows.
set -uo pipefail

out="${1:?usage: diagnostics.sh <output-directory>}"
LOG_LINES="${LOG_LINES:-20000}"
MAX_FILE_BYTES="${MAX_FILE_BYTES:-10485760}"
NAMESPACE="${KYVERNO_NAMESPACE:-kyverno}"
# The deadline of each API request, and of each whole kubectl call.
REQUEST_TIMEOUT="${REQUEST_TIMEOUT:-30s}"
CALL_TIMEOUT="${CALL_TIMEOUT:-60s}"
# Policy and report kinds come from these API groups. The admissionregistration
# group adds ValidatingAdmissionPolicies, MutatingAdmissionPolicies, and their
# bindings, which Kyverno generates.
GROUPS_IN_USE=(kyverno.io policies.kyverno.io reports.kyverno.io wgpolicyk8s.io openreports.io admissionregistration.k8s.io)

mkdir -p "$out/logs"

# kube runs kubectl with deadlines: --request-timeout bounds each API request,
# and timeout, when it's available, bounds the whole call, such as a log
# stream that stalls after it started.
kube() {
  if command -v timeout > /dev/null; then
    timeout --kill-after=10s "$CALL_TIMEOUT" kubectl --request-timeout="$REQUEST_TIMEOUT" "$@"
  else
    kubectl --request-timeout="$REQUEST_TIMEOUT" "$@"
  fi
}

# capture <file> <kubectl arguments...> writes the output and errors of a
# kubectl call, cut at MAX_FILE_BYTES. A call that fails or times out keeps
# its partial output, followed by a "(command failed: ...)" line.
capture() {
  local file="$out/$1"
  shift
  { kube "$@" 2>&1 || echo "(command failed: kubectl $*)"; } | head -c "$MAX_FILE_BYTES" > "$file"
}

if ! kube version > "$out/cluster.txt" 2>&1; then
  echo "The cluster isn't reachable; only the client details were collected."
  exit 0
fi

capture nodes.txt get nodes -o wide
capture pods.txt get pods -A -o wide
capture kyverno-describe.txt -n "$NAMESPACE" describe pods,deployments,services,endpoints
capture kyverno-configmaps.yaml -n "$NAMESPACE" get configmaps -o yaml
capture events.txt get events -A --sort-by=.lastTimestamp -o wide
capture webhooks.yaml get validatingwebhookconfigurations,mutatingwebhookconfigurations -o yaml
capture apiservices.txt get apiservices

# Logs from every container of every Kyverno pod, including the previous run
# of a container that restarted.
for pod in $(kube -n "$NAMESPACE" get pods -o name 2> /dev/null); do
  name="${pod#pod/}"
  capture "logs/$name.log" -n "$NAMESPACE" logs "$pod" --all-containers --prefix --timestamps --tail="$LOG_LINES"
  if kube -n "$NAMESPACE" logs "$pod" --all-containers --previous --tail=1 > /dev/null 2>&1; then
    capture "logs/$name.previous.log" -n "$NAMESPACE" logs "$pod" --all-containers --prefix --timestamps --previous --tail="$LOG_LINES"
  fi
done

# Every resource of the policy and report groups, with its status.
for group in "${GROUPS_IN_USE[@]}"; do
  for resource in $(kube api-resources --api-group="$group" --verbs=list -o name 2> /dev/null); do
    capture "$resource.yaml" get "$resource" -A -o yaml
  done
done

du -sh "$out" 2> /dev/null || true
