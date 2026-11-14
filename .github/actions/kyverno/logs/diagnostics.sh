#!/usr/bin/env bash
# Collects the state of a failed E2E cluster into a directory, for upload as a
# diagnostics artifact: logs from all Kyverno pods, events, webhook
# configurations, policies with their status, and reports.
#
# Usage: diagnostics.sh <output-directory>
#
# Every command is best effort, because the cluster may be broken or missing.
# The output stays bounded: each log keeps its last LOG_LINES lines, and each
# file is cut at MAX_FILE_BYTES.
set -uo pipefail

out="${1:?usage: diagnostics.sh <output-directory>}"
LOG_LINES="${LOG_LINES:-20000}"
MAX_FILE_BYTES="${MAX_FILE_BYTES:-10485760}"
NAMESPACE="${KYVERNO_NAMESPACE:-kyverno}"
# Policy and report kinds come from these API groups. The admissionregistration
# group adds ValidatingAdmissionPolicies, MutatingAdmissionPolicies, and their
# bindings, which Kyverno generates.
GROUPS_IN_USE=(kyverno.io policies.kyverno.io reports.kyverno.io wgpolicyk8s.io openreports.io admissionregistration.k8s.io)

mkdir -p "$out/logs"

# capture <file> <command...> writes the output and errors of a command,
# cut at MAX_FILE_BYTES.
capture() {
  local file="$out/$1"
  shift
  { "$@" 2>&1 || echo "(command failed: $*)"; } | head -c "$MAX_FILE_BYTES" > "$file"
}

if ! kubectl version --request-timeout=10s > "$out/cluster.txt" 2>&1; then
  echo "The cluster isn't reachable; only the client details were collected."
  exit 0
fi

capture nodes.txt kubectl get nodes -o wide
capture pods.txt kubectl get pods -A -o wide
capture kyverno-describe.txt kubectl -n "$NAMESPACE" describe pods,deployments,services,endpoints
capture kyverno-configmaps.yaml kubectl -n "$NAMESPACE" get configmaps -o yaml
capture events.txt kubectl get events -A --sort-by=.lastTimestamp -o wide
capture webhooks.yaml kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations -o yaml
capture apiservices.txt kubectl get apiservices

# Logs from every container of every Kyverno pod, including the previous run
# of a container that restarted.
for pod in $(kubectl -n "$NAMESPACE" get pods -o name 2>/dev/null); do
  name="${pod#pod/}"
  capture "logs/$name.log" kubectl -n "$NAMESPACE" logs "$pod" --all-containers --prefix --timestamps --tail="$LOG_LINES"
  if kubectl -n "$NAMESPACE" logs "$pod" --all-containers --previous --tail=1 > /dev/null 2>&1; then
    capture "logs/$name.previous.log" kubectl -n "$NAMESPACE" logs "$pod" --all-containers --prefix --timestamps --previous --tail="$LOG_LINES"
  fi
done

# Every resource of the policy and report groups, with its status.
for group in "${GROUPS_IN_USE[@]}"; do
  for resource in $(kubectl api-resources --api-group="$group" --verbs=list -o name 2>/dev/null); do
    capture "$resource.yaml" kubectl get "$resource" -A -o yaml
  done
done

du -sh "$out" 2>/dev/null || true
