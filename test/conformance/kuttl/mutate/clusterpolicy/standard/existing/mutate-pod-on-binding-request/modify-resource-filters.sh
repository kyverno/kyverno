#!/usr/bin/env bash
set -eu

if [ $# -ne 1 ]; then
  echo "Usage: $0 [addBinding|removeBinding]"
  exit 1
fi

if [ "$1" = "removeBinding" ]; then
  resource_filters=$(kubectl get ConfigMap kyverno -n kyverno -o json | jq .data.resourceFilters)
  resource_filters="${resource_filters//\[Pod\/binding,\*,\*\]/}"

  kubectl patch ConfigMap kyverno -n kyverno --type='json' -p="[{\"op\": \"replace\", \"path\": \"/data/resourceFilters\", \"value\":""$resource_filters""}]"
fi

if [ "$1" = "addBinding" ]; then
  resource_filters=$(kubectl get ConfigMap kyverno -n kyverno -o json | jq .data.resourceFilters)
  resource_filters="${resource_filters%?}"

  resource_filters="${resource_filters}""[Pod/binding,*,*]\""
  kubectl patch ConfigMap kyverno -n kyverno --type='json' -p="[{\"op\": \"replace\", \"path\": \"/data/resourceFilters\", \"value\":""$resource_filters""}]"
fi
