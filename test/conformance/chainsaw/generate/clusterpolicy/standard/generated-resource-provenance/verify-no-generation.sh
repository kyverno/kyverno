#!/usr/bin/env bash
set -euo pipefail

mode=${1:?operation is required}
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT
selector=generate.kyverno.io/policy-name=generated-resource-provenance
update_uid=$(kubectl get namespace generated-resource-provenance-update -o jsonpath='{.metadata.uid}')
delete_uid=$(kubectl get namespace generated-resource-provenance-delete -o jsonpath='{.metadata.uid}')
candidate_uid=$(kubectl get namespace generated-resource-provenance-candidate -o jsonpath='{.metadata.uid}')

# Watch from the list resourceVersion, so even a short lived request created
# during the operation is observed. The genuine control has a different UID.
forged_trigger_filter='select(any(.spec.ruleContext[]?; .trigger.uid == $update_uid or .trigger.uid == $delete_uid or .trigger.uid == $candidate_uid))'
kubectl get updaterequests -A -l "$selector" -o json > "$work_dir/before.json"
jq --arg update_uid "$update_uid" --arg delete_uid "$delete_uid" --arg candidate_uid "$candidate_uid" \
  ".items[] | $forged_trigger_filter" "$work_dir/before.json" > "$work_dir/matches.json"
test ! -s "$work_dir/matches.json"
resource_version=$(jq -er '.metadata.resourceVersion' "$work_dir/before.json")

case "$mode" in
  update)
    # A real PUT reaches admission even though all labels and data are unchanged.
    kubectl get secret synchronized -n generated-resource-provenance-update -o json | kubectl replace -f -
    ;;
  delete)
    kubectl delete secret synchronized -n generated-resource-provenance-delete
    ;;
  source)
    kubectl patch secret source -n generated-resource-provenance-source --type=merge -p '{"data":{"value":"Y2hhbmdlZA=="}}'
    ;;
  restore-source)
    # User owned sources are replaceable without retaining the controller label.
    kubectl get secret source -n generated-resource-provenance-source -o json | \
      jq 'del(.metadata.labels["generate.kyverno.io/clone-source"])' | kubectl replace -f -
    # Restoring the marker is allowed too; delete then traverses clone sources.
    kubectl label secret source -n generated-resource-provenance-source generate.kyverno.io/clone-source= --overwrite
    # Backup restores are also allowed to carry the clone-source marker.
    kubectl delete secret source -n generated-resource-provenance-source
    kubectl create -f source-restored.yaml
    ;;
  copied-provenance)
    provenance=$(kubectl get secret synchronized -n generated-resource-provenance-genuine \
      -o go-template='{{index .metadata.annotations "generate.kyverno.io/provenance"}}')
    test -n "$provenance"
    test "$provenance" != '<no value>'
    if kubectl annotate secret synchronized -n generated-resource-provenance-update \
      "generate.kyverno.io/provenance=$provenance" --overwrite > "$work_dir/annotate.stdout" 2> "$work_dir/annotate.stderr"; then
      echo "A user was allowed to copy Kyverno provenance onto another object" >&2
      exit 1
    fi
    if ! grep -Fq 'Kyverno generate provenance can only be set by Kyverno' "$work_dir/annotate.stderr"; then
      cat "$work_dir/annotate.stderr" >&2
      exit 1
    fi
    ;;
  *)
    echo "Unknown operation: $mode" >&2
    exit 1
    ;;
esac

selector_query=$(jq -nr --arg selector "$selector" '$selector | @uri')
watch_url="/apis/kyverno.io/v2/updaterequests?watch=true&resourceVersion=${resource_version}&labelSelector=${selector_query}&timeoutSeconds=10"
# Use the raw watch API: kubectl get does not expose a resourceVersion flag.
# The server closes a successful watch after ten seconds; the outer timeout is
# a watchdog, not a success condition.
watch_started=$(date +%s)
if ! timeout 15s kubectl get --raw "$watch_url" > "$work_dir/events.json" 2> "$work_dir/watch.stderr"; then
  cat "$work_dir/watch.stderr" >&2
  echo "UpdateRequest watch failed during $mode" >&2
  exit 1
fi
watch_elapsed=$(( $(date +%s) - watch_started ))
if [ "$watch_elapsed" -lt 9 ]; then
  echo "UpdateRequest watch closed prematurely after ${watch_elapsed}s" >&2
  exit 1
fi
jq 'select(.type == "ERROR" or (.kind == "Status" and .status == "Failure") or (.object.kind == "Status" and .object.status == "Failure"))' "$work_dir/events.json" > "$work_dir/watch-errors.json"
if [ -s "$work_dir/watch-errors.json" ]; then
  cat "$work_dir/watch-errors.json" >&2
  exit 1
fi
jq --arg update_uid "$update_uid" --arg delete_uid "$delete_uid" --arg candidate_uid "$candidate_uid" \
  ".object | $forged_trigger_filter" "$work_dir/events.json" > "$work_dir/matches.json"
if [ -s "$work_dir/matches.json" ]; then
  cat "$work_dir/matches.json" >&2
  echo "A planted downstream enqueued an UpdateRequest during $mode" >&2
  exit 1
fi
