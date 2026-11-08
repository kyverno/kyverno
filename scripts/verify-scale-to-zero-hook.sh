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

# kyverno.fullname is capped at 63 chars. The pre-delete hooks append a suffix to it, so
# a long fullname is shortened first, keeping a hash of the full name: otherwise the suffix
# is cut off, the Job name can exceed the 63 chars allowed in the job-name Pod label, and
# releases whose fullnames share a long prefix would get the same hook resources.
# Prints "<kind> <metadata.name>" for every object of both hooks rendered for a fullname.
render_hook_objects() {
  local fullname="$1" render
  render="$("${HELM}" template kyverno "${ROOT_DIR}/charts/kyverno" \
    --namespace kyverno --kube-version "${KUBE_VERSION}" \
    --set fullnameOverride="${fullname}" \
    --show-only templates/hooks/pre-delete-scale-to-zero.yaml \
    --show-only templates/hooks/pre-delete-remove-webhooks.yaml)"
  awk '
    /^kind: / { kind = $2 }
    /^metadata:/ { meta = 1; next }
    meta && /^  name: / { print kind " " $2; meta = 0 }
  ' <<< "${render}"
}

verify_long_fullname_hook_names() {
  local fullname="$1" got hash want
  got="$(render_hook_objects "${fullname}")"
  hash="$(printf '%s' "${fullname}" | sha256sum | cut -c1-8)"
  while IFS= read -r want; do
    if ! grep -Fxq -- "${want}" <<< "${got}"; then
      printf 'Missing or wrong hook object for fullname %s: %s\nRendered:\n%s\n' "${fullname}" "${want}" "${got}" >&2
      return 1
    fi
  done <<EOF
ServiceAccount ${fullname:0:40}-${hash}-scale-to-zero
Job ${fullname:0:40}-${hash}-scale-to-zero
Role ${fullname:0:40}-${hash}:scale-to-zero
ServiceAccount ${fullname:0:42}-${hash}-rm-webhooks
Job ${fullname:0:42}-${hash}-rm-webhooks
ClusterRole ${fullname:0:42}-${hash}:rm-webhooks
EOF
  if awk '{ if (length($2) > 63) bad = 1 } END { exit !bad }' <<< "${got}"; then
    printf 'A hook object name exceeds 63 chars:\n%s\n' "${got}" >&2
    return 1
  fi
}

long_a="$(head -c 63 /dev/zero | tr '\0' 'k')"
long_b="${long_a:0:62}j"
verify_long_fullname_hook_names "${long_a}"
verify_long_fullname_hook_names "${long_b}"
sa_a="$(render_hook_objects "${long_a}" | grep -m1 '^ServiceAccount ')"
sa_b="$(render_hook_objects "${long_b}" | grep -m1 '^ServiceAccount ')"
if [[ "${sa_a}" == "${sa_b}" ]]; then
  printf 'Releases with fullnames sharing a 62-char prefix got the same hook ServiceAccount: %s\n' "${sa_a}" >&2
  exit 1
fi
printf 'Hook names stay unique and within 63 chars for long fullnames\n'

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
