#!/usr/bin/env bash
set -euo pipefail

trigger_namespace=gpol-label-ownership
namespace=gpol-label-ownership-target
user="system:serviceaccount:${trigger_namespace}:label-editor"

as_user() {
  kubectl --as="$user" --namespace="$namespace" "$@"
}

expect_denied() {
  local output
  if output=$(as_user "$@" 2>&1); then
    printf 'Expected generation-label rejection, but the request succeeded: %s\n' "$*" >&2
    exit 1
  fi
  if [[ "$output" != *"generate labels can only be set by Kyverno"* ]]; then
    printf 'Unexpected failure for %s:\n%s\n' "$*" "$output" >&2
    exit 1
  fi
}

# The controller stamps the identity of the actual trigger on its downstream.
trigger_uid=$(kubectl --as="$user" --namespace="$trigger_namespace" get configmap trigger -o jsonpath='{.metadata.uid}')
downstream_trigger_uid=$(as_user get secret generated -o go-template='{{ index .metadata.labels "generate.kyverno.io/trigger-uid" }}')
[[ -n "$trigger_uid" && "$downstream_trigger_uid" == "$trigger_uid" ]]

# The target is outside the trigger kind, namespace and object selector.
# The editor has ordinary Secret write permissions, including CREATE.
as_user create secret generic ordinary --from-literal=value=original
expect_denied create -f reserved-create.yaml
expect_denied label secret ordinary generate.kyverno.io/policy-name=gpol-label-ownership
expect_denied label secret generated generate.kyverno.io/policy-name=changed --overwrite
expect_denied label secret generated generate.kyverno.io/policy-name-
expect_denied label secret generated app.kubernetes.io/managed-by=another-controller --overwrite

# Retaining routing metadata allows users to edit a non-synchronized downstream.
as_user patch secret generated --type=merge \
  -p '{"metadata":{"labels":{"example.com/user":"edited"}},"data":{"value":"ZWRpdGVk"}}'
[[ "$(as_user get secret generated -o jsonpath='{.data.value}')" == ZWRpdGVk ]]
[[ "$(as_user get secret generated -o go-template='{{ index .metadata.labels "generate.kyverno.io/policy-name" }}')" == gpol-label-ownership ]]
[[ "$(as_user get secret generated -o go-template='{{ index .metadata.labels "example.com/user" }}')" == edited ]]

# Clone-source markers and standalone managed-by labels remain user-editable.
as_user create -f user-owned.yaml
as_user label configmap user-owned generate.kyverno.io/clone-source=restored --overwrite
as_user label configmap user-owned generate.kyverno.io/clone-source-
as_user label configmap user-owned app.kubernetes.io/managed-by=another-controller --overwrite
as_user label configmap user-owned app.kubernetes.io/managed-by-
as_user patch configmap user-owned --type=merge -p '{"data":{"value":"replaced"}}'
[[ "$(as_user get configmap user-owned -o jsonpath='{.data.value}')" == replaced ]]
