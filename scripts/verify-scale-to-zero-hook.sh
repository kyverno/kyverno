#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELM="${HELM:-helm}"
KUBE_VERSION="${KUBE_VERSION:-v1.25.0}"

verify() {
  local release="$1" namespace="$2" expected_namespace="$3" fullname="$4"
  shift 4
  local render line reading=false
  local -a args=()
  render="$("${HELM}" template "${release}" "${ROOT_DIR}/charts/kyverno" \
    --namespace "${namespace}" --kube-version "${KUBE_VERSION}" \
    --show-only templates/hooks/pre-delete-scale-to-zero.yaml "$@")"
  while IFS= read -r line; do
    if [[ "${line}" == '          args:' ]]; then
      reading=true
    elif [[ "${reading}" == true && "${line}" == '            - '* ]]; then
      args+=("${line#'            - '}")
    elif [[ "${reading}" == true ]]; then
      break
    fi
  done <<< "${render}"
  if [[ "${#args[@]}" -ne 3 || "${args[0]-}" != "scale-deploy" ||
        "${args[1]-}" != "--namespace=${expected_namespace}" ||
        "${args[2]-}" != "--label=app.kubernetes.io/part-of=${fullname}" ]]; then
    printf 'Unexpected scale-to-zero args for %s: %s\n' "${release}" "${args[*]}" >&2
    return 1
  fi
}

verify kyverno kyverno kyverno kyverno
verify kyverno-prod kyverno-system kyverno-system kyverno-prod
verify kyverno-prod release-ns workload-ns kyverno-prod --set namespaceOverride=workload-ns
verify kyverno-prod release-ns release-ns custom-kyverno --set fullnameOverride=custom-kyverno
verify kyverno-prod release-ns workload-ns custom-kyverno --set namespaceOverride=workload-ns --set fullnameOverride=custom-kyverno
printf 'Scale-to-zero release scope verified\n'

verify_image_registry() {
  local release="$1" expected_image="$2"
  shift 2
  local render image
  render="$("${HELM}" template "${release}" "${ROOT_DIR}/charts/kyverno" \
    --namespace kyverno --kube-version "${KUBE_VERSION}" \
    --show-only templates/hooks/pre-delete-scale-to-zero.yaml "$@")"
  image="$(grep -m1 '^\s*image:' <<< "${render}" | sed -E 's/^[[:space:]]*image:[[:space:]]*"?([^"]*)"?[[:space:]]*$/\1/')"
  if [[ "${image}" != "${expected_image}" ]]; then
    printf 'Unexpected webhooksCleanup hook image for %s: got %q want %q\n' "${release}" "${image}" "${expected_image}" >&2
    return 1
  fi
}

verify_image_registry kyverno "ghcr.io/kyverno/readiness-checker:latest"
verify_image_registry kyverno "my-mirror.example.com/kyverno/readiness-checker:latest" \
  --set global.image.registry=my-mirror.example.com
verify_image_registry kyverno "explicit.example.com/kyverno/readiness-checker:latest" \
  --set global.image.registry=my-mirror.example.com \
  --set webhooksCleanup.image.registry=explicit.example.com
printf 'Webhooks-cleanup image registry propagation verified\n'
