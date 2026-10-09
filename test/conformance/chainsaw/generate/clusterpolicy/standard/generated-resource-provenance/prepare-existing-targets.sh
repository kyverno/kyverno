#!/usr/bin/env bash
set -euo pipefail

# Create exact intended downstream names while no sync policy watches Secrets.
# Both the trigger and clone source identities refer to real existing objects.
source_uid=$(kubectl get secret source -n generated-resource-provenance-source -o jsonpath='{.metadata.uid}')
for suffix in update delete candidate; do
  namespace="generated-resource-provenance-${suffix}"
  trigger_uid=$(kubectl get namespace "$namespace" -o jsonpath='{.metadata.uid}')
  kubectl create secret generic synchronized -n "$namespace" --from-literal=value=spoofed
  kubectl label secret synchronized -n "$namespace" \
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
kubectl annotate secret synchronized -n generated-resource-provenance-delete \
  generate.kyverno.io/provenance=v1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
