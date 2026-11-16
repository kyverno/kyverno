#!/usr/bin/env bash
# Collects diagnostics from a cluster running Kyverno into a directory, one file
# per item, so a failed CI job can upload them before its cluster is gone.
#
# Every kubectl command is best effort and has a timeout. A command that fails
# or hangs is listed in README.md and the rest still runs, so the script only
# fails when it can't write to the output directory.
#
# Secrets are never read. UpdateRequests are listed as a table and not dumped,
# because their spec carries the admission request, and with it the resource
# that triggered the rule, which can be a Secret.
#
# Usage: diagnostics.sh <output-dir>
#   KYVERNO_NAMESPACE    namespace Kyverno runs in (default kyverno)
#   DIAGNOSTICS_TIMEOUT  seconds one kubectl command may take (default 30)
#   DIAGNOSTICS_BUDGET   seconds all commands together may take (default 300)
set -uo pipefail

out=${1:?usage: diagnostics.sh <output-dir>}
namespace=${KYVERNO_NAMESPACE:-kyverno}
timeout=${DIAGNOSTICS_TIMEOUT:-30}
budget=${DIAGNOSTICS_BUDGET:-300}

mkdir -p "$out" || exit 1
work=$(mktemp -d) || exit 1
trap 'rm -rf "$work"' EXIT
: > "$work/collected"
: > "$work/failed"

# GNU timeout is always there on the runners; perl covers machines without it.
if command -v timeout > /dev/null 2>&1; then
  with_timeout() { timeout -k 5 "$@"; }
else
  with_timeout() { perl -e 'alarm shift; exec @ARGV' "$@"; }
fi

# capture <file> <kubectl arguments>: saves the output of a kubectl command.
capture() {
  local file=$1 rc=0 left=$((budget - SECONDS)) reason
  shift
  if [ "$left" -le 0 ]; then
    echo "- $file: kubectl $*: skipped, the ${budget}s budget was used up" >> "$work/failed"
    return 0
  fi
  [ "$left" -lt "$timeout" ] || left=$timeout
  mkdir -p "$(dirname "$out/$file")"
  with_timeout "$left" kubectl "$@" < /dev/null > "$out/$file" 2> "$work/stderr" || rc=$?
  if [ "$rc" -eq 0 ]; then
    echo "- $file: kubectl $*" >> "$work/collected"
    return 0
  fi
  # Keep partial output, it can still help.
  [ -s "$out/$file" ] || rm -f "$out/$file"
  case $rc in
    124 | 137 | 142) reason="timed out after ${left}s" ;;
    *) reason="exit code $rc: $(head -n 1 "$work/stderr")" ;;
  esac
  echo "- $file: kubectl $*: $reason" >> "$work/failed"
}

capture cluster/version.txt version
capture cluster/nodes.txt get nodes -o wide
capture cluster/pods.txt get pods -A -o wide
# Sorted by creation time and not lastTimestamp, which is empty for events
# created through events.k8s.io, as Kyverno does.
capture cluster/events.txt get events -A --sort-by=.metadata.creationTimestamp \
  -o 'custom-columns=CREATED:.metadata.creationTimestamp,NAMESPACE:.metadata.namespace,TYPE:.type,REASON:.reason,KIND:.involvedObject.kind,OBJECT:.involvedObject.name,FROM:.reportingComponent,MESSAGE:.message'

# The CA bundle in a webhook configuration is public.
capture webhooks/validatingwebhookconfigurations.yaml get validatingwebhookconfigurations -o yaml
capture webhooks/mutatingwebhookconfigurations.yaml get mutatingwebhookconfigurations -o yaml

capture kyverno/pods.txt -n "$namespace" get pods -o wide
# kubectl 1.36+ hides events when describing several objects.
capture kyverno/describe-pods.txt -n "$namespace" describe pods --show-events=true
# One line per container: pod, container, restart count.
capture kyverno/containers.txt -n "$namespace" get pods \
  -o 'go-template={{range .items}}{{$pod := .metadata.name}}{{range .status.initContainerStatuses}}{{$pod}} {{.name}} {{.restartCount}}{{"\n"}}{{end}}{{range .status.containerStatuses}}{{$pod}} {{.name}} {{.restartCount}}{{"\n"}}{{end}}{{end}}'
if [ -f "$out/kyverno/containers.txt" ]; then
  while read -r pod container restarts; do
    [ -n "$container" ] || continue
    capture "kyverno/logs/$pod/$container.log" -n "$namespace" logs "$pod" -c "$container"
    if [ "${restarts:-0}" -gt 0 ] 2> /dev/null; then
      capture "kyverno/logs/$pod/$container.previous.log" -n "$namespace" logs "$pod" -c "$container" --previous
    fi
  done < "$out/kyverno/containers.txt"
fi

# Kinds are discovered and not listed here, so a new policy or report kind is
# picked up without a change. A group that isn't installed has no kinds.
capture cluster/api-resources.txt api-resources -o name
if [ -f "$out/cluster/api-resources.txt" ]; then
  while read -r resource; do
    case $resource in
      updaterequests.kyverno.io)
        capture policies/updaterequests.kyverno.io.txt get "$resource" -A \
          -o 'custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,CREATED:.metadata.creationTimestamp,TYPE:.spec.requestType,POLICY:.spec.policy,RULES:.spec.ruleContext[*].rule,TRIGGER-KINDS:.spec.ruleContext[*].trigger.kind,TRIGGER-NAMESPACES:.spec.ruleContext[*].trigger.namespace,TRIGGER-NAMES:.spec.ruleContext[*].trigger.name,STATE:.status.state,RETRIES:.status.retryCount,MESSAGE:.status.message'
        continue
        ;;
      # Already collected above.
      *webhookconfigurations.admissionregistration.k8s.io) continue ;;
    esac
    # The API group is what follows the first dot.
    case ${resource#*.} in
      kyverno.io | policies.kyverno.io | admissionregistration.k8s.io) dir=policies ;;
      reports.kyverno.io | wgpolicyk8s.io | openreports.io) dir=reports ;;
      *) continue ;;
    esac
    capture "$dir/$resource.yaml" get "$resource" -A -o yaml
  done < "$out/cluster/api-resources.txt"
fi

{
  echo "# Kyverno diagnostics"
  echo
  echo "Collected at $(date -u +%Y-%m-%dT%H:%M:%SZ) by .github/actions/kyverno/logs/diagnostics.sh."
  if [ -n "${GITHUB_RUN_ID:-}" ]; then
    echo "Run: ${GITHUB_SERVER_URL:-}/${GITHUB_REPOSITORY:-}/actions/runs/$GITHUB_RUN_ID, attempt ${GITHUB_RUN_ATTEMPT:-}, job ${GITHUB_JOB:-}."
  fi
  echo
  echo "Secrets are not collected. UpdateRequests are listed without the admission request they carry."
  echo
  echo "## Collected"
  echo
  cat "$work/collected"
  echo
  echo "## Failed"
  echo
  if [ -s "$work/failed" ]; then cat "$work/failed"; else echo "Nothing failed."; fi
} > "$out/README.md"

echo "Collected $(wc -l < "$work/collected" | tr -d ' ') files into $out, $(wc -l < "$work/failed" | tr -d ' ') commands failed."
