#!/usr/bin/env bash
set -euo pipefail

# Only fixture setup impersonates the configured background controller, allowing
# us to model persisted unsigned objects with the same downstream routing labels.
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

for rule in direct foreach; do
  kubectl get configmap "${rule}-source" -n clone-kind-cleanup-target -o json \
    | jq --arg name "unsigned-${rule}" '{
        apiVersion, kind,
        metadata: {name: $name, namespace: .metadata.namespace, labels: .metadata.labels},
        data: {value: "unsigned"}
      }' \
    | kubectl --as="$background_username" create -f -
done
