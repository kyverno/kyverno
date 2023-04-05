#!/usr/bin/env bash
set -eu

if [ $# -ne 1 ]; then
  echo "Usage: $0 [addNode|removeNode]"
  exit 1
fi

if [ "$1" = "removeNode" ]; then
  resource_filters=$(kubectl get ConfigMap kyverno -n kyverno -o json | jq .data.resourceFilters)
  resource_filters="${resource_filters//\[Node,\*,\*\]/}"
  resource_filters="${resource_filters//\[Node\/\*,\*,\*\]/}"

  kubectl patch ConfigMap kyverno -n kyverno --type='json' -p="[{\"op\": \"replace\", \"path\": \"/data/resourceFilters\", \"value\":""$resource_filters""}]"
fi

if [ "$1" = "addNode" ]; then
  resource_filters=$(kubectl get ConfigMap kyverno -n kyverno -o json | jq .data.resourceFilters)
  resource_filters="${resource_filters%?}"

  resource_filters="${resource_filters}""[Node,*,*][Node/*,*,*]\""
  kubectl patch ConfigMap kyverno -n kyverno --type='json' -p="[{\"op\": \"replace\", \"path\": \"/data/resourceFilters\", \"value\":""$resource_filters""}]"
fi
