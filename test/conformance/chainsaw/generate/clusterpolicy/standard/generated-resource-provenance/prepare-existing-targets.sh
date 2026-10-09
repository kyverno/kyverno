#!/usr/bin/env bash
set -euo pipefail

# Administrative setup simulates metadata left by an earlier installation.
# Discover the configured identity rather than assuming a default account name.
controller_namespace=${KYVERNO_NAMESPACE:-kyverno}
background_service_account=$(kubectl get deployments -n "$controller_namespace" \
  -l app.kubernetes.io/component=background-controller -o json | jq -er '
  if (.items | length) != 1 then
    error("expected exactly one background-controller Deployment")
  else
    .items[0].spec.template.spec.serviceAccountName
  end
  | if type == "string" and length > 0 then .
    else error("background-controller Deployment must specify a service account")
    end')
background_username="system:serviceaccount:${controller_namespace}:${background_service_account}"

# The fixture's aggregated role may take a moment to reach the controller role.
deadline=$((SECONDS + 10))
until kubectl --as="$background_username" auth can-i patch secrets \
  -n generated-resource-provenance-update --quiet --request-timeout=2s; do
  if (( SECONDS >= deadline )); then
    printf 'Background controller Secret permissions did not become ready\n' >&2
    exit 1
  fi
  sleep 1
done

# Create exact intended downstream names while no sync policy watches Secrets.
# Both the trigger and clone source identities refer to real existing objects.
source_uid=$(kubectl get secret source -n generated-resource-provenance-source -o jsonpath='{.metadata.uid}')
for suffix in update delete candidate; do
  namespace="generated-resource-provenance-${suffix}"
  trigger_uid=$(kubectl get namespace "$namespace" -o jsonpath='{.metadata.uid}')
  kubectl create secret generic synchronized -n "$namespace" --from-literal=value=spoofed
  kubectl --as="$background_username" label secret synchronized -n "$namespace" \
    app.kubernetes.io/managed-by=kyverno \
    generate.kyverno.io/policy-name=generated-resource-provenance \
    generate.kyverno.io/policy-namespace= \
    generate.kyverno.io/rule-name=sync-secret \
    generate.kyverno.io/trigger-group= \
    generate.kyverno.io/trigger-version=v1 \
    generate.kyverno.io/trigger-kind=Namespace \
    generate.kyverno.io/trigger-namespace= \
    "generate.kyverno.io/trigger-name=$namespace" \
    "generate.kyverno.io/trigger-uid=$trigger_uid" \
    generate.kyverno.io/source-group= \
    generate.kyverno.io/source-version=v1 \
    generate.kyverno.io/source-kind=Secret \
    generate.kyverno.io/source-namespace=generated-resource-provenance-source \
    generate.kyverno.io/source-name=source \
    "generate.kyverno.io/source-uid=$source_uid"
done
# A syntactically valid but unauthenticated MAC must not establish provenance.
kubectl --as="$background_username" annotate secret synchronized -n generated-resource-provenance-delete \
  generate.kyverno.io/provenance=v1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
