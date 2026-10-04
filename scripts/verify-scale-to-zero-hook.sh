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
