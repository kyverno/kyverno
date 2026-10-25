#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELM="${HELM:-helm}"
KUBE_VERSION="${KUBE_VERSION:-v1.25.0}"

# The NetworkPolicy has to allow the port the metrics server listens on (metering.port),
# not the Service port, or scrapes are dropped when the two differ.
verify() {
  local controller="$1" render
  render="$("${HELM}" template kyverno "${ROOT_DIR}/charts/kyverno" \
    --namespace kyverno --kube-version "${KUBE_VERSION}" \
    --show-only "templates/${controller}-controller/networkpolicy.yaml" \
    --set "${controller}Controller.networkPolicy.enabled=true" \
    --set "${controller}Controller.networkPolicy.ingressFrom[0].podSelector.matchLabels.app=prometheus" \
    --set "${controller}Controller.metricsService.port=8000" \
    --set "${controller}Controller.metering.port=9000")"
  if ! grep -Eq '^ +port: 9000$' <<< "${render}" || grep -Eq '^ +port: 8000$' <<< "${render}"; then
    printf 'NetworkPolicy for the %s controller does not allow metering.port:\n%s\n' "${controller}" "${render}" >&2
    return 1
  fi
}

for controller in admission background cleanup reports; do
  verify "${controller}"
done
printf 'NetworkPolicy metrics ports verified\n'
