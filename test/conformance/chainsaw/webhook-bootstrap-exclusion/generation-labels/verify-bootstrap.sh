#!/usr/bin/env bash
set -euo pipefail

namespace=${KYVERNO_NAMESPACE:-kyverno}
configuration=kyverno-resource-validating-webhook-cfg
protection=protect-generation-labels.kyverno.svc
probe_name="generation-bootstrap-$(date +%s)-$$"
manual_name="${probe_name}.manual.example.com"
probe_created=false
manual_created=false
args_changed=false
temp_dir=$(mktemp -d)

deployment=$(kubectl -n "$namespace" get deployment -l app.kubernetes.io/component=admission-controller -o json | jq -er '.items | if length == 1 then .[0].metadata.name else error("expected one admission deployment") end')
kubectl -n "$namespace" get deployment "$deployment" -o json > "$temp_dir/deployment.json"
container=$(jq -er '.spec.template.spec.containers[] | select(.name == "kyverno") | .name' "$temp_dir/deployment.json")
# Omitted autoUpdateWebhooks defaults to true. Do not rewrite an installation's
# pre-existing manual routing by temporarily enabling automatic reconciliation.
if ! jq -e --arg container "$container" '[.spec.template.spec.containers[] | select(.name == $container) | .args[]? | select(startswith("--autoUpdateWebhooks"))] | all(. == "--autoUpdateWebhooks=true")' "$temp_dir/deployment.json" >/dev/null; then
  printf 'This test requires an initially automatic webhook installation\n' >&2
  rm -rf "$temp_dir"
  exit 1
fi
jq --arg container "$container" '{spec:{template:{spec:{containers:[.spec.template.spec.containers[] | select(.name == $container) | {name,args}]}}}}' "$temp_dir/deployment.json" > "$temp_dir/original-args.json"

cleanup() {
  local status=$?
  trap - EXIT
  set +e
  if [[ "$probe_created" == true ]]; then
    kubectl delete validatingwebhookconfiguration "$probe_name" --ignore-not-found || status=1
  fi
  if [[ "$manual_created" == true ]]; then
    local removed=false
    for ((attempt=0; attempt<10; attempt++)); do
      if kubectl get validatingwebhookconfiguration "$configuration" -o json > "$temp_dir/cleanup.json" &&
         jq --arg name "$manual_name" '[{op:"test",path:"/metadata/resourceVersion",value:.metadata.resourceVersion},{op:"replace",path:"/webhooks",value:[.webhooks[] | select(.name != $name)]}]' "$temp_dir/cleanup.json" > "$temp_dir/cleanup-patch.json" &&
         kubectl patch validatingwebhookconfiguration "$configuration" --type=json --patch-file "$temp_dir/cleanup-patch.json"; then
        removed=true
        break
      fi
      sleep 1
    done
    [[ "$removed" == true ]] || status=1
  fi
  if [[ "$args_changed" == true ]]; then
    kubectl -n "$namespace" patch deployment "$deployment" --type=strategic --patch-file "$temp_dir/original-args.json" || status=1
    kubectl -n "$namespace" rollout status deployment/"$deployment" --timeout=180s || status=1
  fi
  rm -rf "$temp_dir"
  exit "$status"
}
trap cleanup EXIT

wait_for_exclusion() {
  for ((attempt=0; attempt<45; attempt++)); do
    kubectl get validatingwebhookconfiguration "$configuration" -o json > "$temp_dir/current.json"
    if jq -e --arg name "$protection" '.webhooks[] | select(.name == $name) | .matchConditions | length == 1' "$temp_dir/current.json" >/dev/null; then
      jq --arg name "$protection" '.webhooks[] | select(.name == $name)' "$temp_dir/current.json" > "$temp_dir/emitted.json"
      return
    fi
    sleep 2
  done
  printf 'The generation-label webhook did not acquire the bootstrap exclusion\n' >&2
  return 1
}

expect_dry_run() {
  local expectation=$1 manifest=$2 output
  for ((attempt=0; attempt<30; attempt++)); do
    if output=$(kubectl create --dry-run=server -f "$manifest" 2>&1); then
      if [[ "$expectation" == allowed ]]; then
        return
      fi
    elif [[ "$expectation" == unavailable && "$output" == *"${probe_name}.example.com"* && "$output" == *"service"*"not found"* ]]; then
      return
    fi
    sleep 2
  done
  printf 'Expected %s for %s, last response: %s\n' "$expectation" "$manifest" "$output" >&2
  return 1
}

for automatic in true false; do
  # Build each desired flag set from the original args and restore them on exit.
  jq --arg container "$container" --arg automatic "$automatic" '{spec:{template:{spec:{containers:[.spec.template.spec.containers[] | select(.name == $container) | {name,args:((.args // [] | map(select(startswith("--excludeBootstrapResources=") or startswith("--autoUpdateWebhooks=") | not))) + ["--excludeBootstrapResources=true", "--autoUpdateWebhooks=" + $automatic])}]}}}}' "$temp_dir/deployment.json" > "$temp_dir/args.json"
  args_changed=true
  kubectl -n "$namespace" patch deployment "$deployment" --type=strategic --patch-file "$temp_dir/args.json"
  kubectl -n "$namespace" rollout status deployment/"$deployment" --timeout=180s
  wait_for_exclusion

  if [[ "$automatic" == false ]]; then
    # Simulate an existing manual entry and an older reserved entry together.
    # Reconciliation must refresh only the reserved entry, preserving the manual one.
    jq --arg name "$manual_name" '.name=$name | .failurePolicy="Ignore" | del(.matchConditions) | .objectSelector={matchLabels:{"test.kyverno.io/manual-bootstrap-proof":"true"}}' "$temp_dir/emitted.json" > "$temp_dir/manual.json"
    jq --arg protected "$protection" --slurpfile manual "$temp_dir/manual.json" '[{op:"test",path:"/metadata/resourceVersion",value:.metadata.resourceVersion},{op:"replace",path:"/webhooks",value:([.webhooks[] | if .name == $protected then .matchConditions=[] else . end] + $manual)}]' "$temp_dir/current.json" > "$temp_dir/manual-patch.json"
    manual_created=true
    kubectl patch validatingwebhookconfiguration "$configuration" --type=json --patch-file "$temp_dir/manual-patch.json"
    wait_for_exclusion
    jq -e --arg name "$manual_name" --slurpfile expected "$temp_dir/manual.json" '[.webhooks[] | select(.name == $name)] == $expected' "$temp_dir/current.json" >/dev/null
  fi

  # Copy the emitted rule/selector/exclusion into a narrowly selected webhook
  # whose service does not exist. No controller or persisted Node is stopped.
  jq --arg name "$probe_name" --arg namespace "$namespace" '{apiVersion:"admissionregistration.k8s.io/v1",kind:"ValidatingWebhookConfiguration",metadata:{name:$name},webhooks:[(. | .name=($name + ".example.com") | .clientConfig={service:{namespace:$namespace,name:$name,path:"/validate",port:443}} | .timeoutSeconds=2 | .objectSelector.matchLabels["test.kyverno.io/bootstrap-probe"]="true" | .matchConditions=[])]}' "$temp_dir/emitted.json" > "$temp_dir/probe.json"
  probe_created=true
  kubectl create -f "$temp_dir/probe.json"
  for manifest in node.yaml csr.yaml; do
    expect_dry_run unavailable "$manifest"
  done

  jq '[{op:"add",path:"/webhooks/0/matchConditions",value:.matchConditions}]' "$temp_dir/emitted.json" > "$temp_dir/conditions-patch.json"
  kubectl patch validatingwebhookconfiguration "$probe_name" --type=json --patch-file "$temp_dir/conditions-patch.json"
  for manifest in node.yaml csr.yaml; do
    expect_dry_run allowed "$manifest"
  done
  kubectl delete validatingwebhookconfiguration "$probe_name"
  probe_created=false
  printf 'Bootstrap admission passed with autoUpdateWebhooks=%s and an unavailable callback\n' "$automatic"
done
