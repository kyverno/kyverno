#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELM="${HELM:-helm}"
KUBE_VERSION="${KUBE_VERSION:-v1.25.0}"

APP_VERSION="$(sed -n 's/^appVersion:[[:space:]]*//p' "${ROOT_DIR}/charts/kyverno/Chart.yaml" | sed 's/[[:space:]]*#.*//' | tr -d '"'"'"'' | head -1)"
if [[ -z "${APP_VERSION}" ]]; then
  printf 'Failed to extract appVersion from Chart.yaml\n' >&2
  exit 1
fi

render="$("${HELM}" template kyverno "${ROOT_DIR}/charts/kyverno" \
  --kube-version "${KUBE_VERSION}" --set unittest=true \
  --show-only templates/tests/helper-functions-test.yaml)"

get_value() {
  local key="$1"
  printf '%s\n' "${render}" | sed -n "s/^  ${key}: //p"
}

check() {
  local key="$1" expected="$2" actual
  actual="$(get_value "${key}")"
  if [[ "${actual}" != "${expected}" ]]; then
    printf 'Unexpected %s: got %q, want %q\n' "${key}" "${actual}" "${expected}" >&2
    return 1
  fi
}

# test.image always falls back to a literal "latest" tag (kyverno.test.image
# hardcodes defaultTag), regardless of Chart.AppVersion.
check testImageDefaultRegistry "ghcr.io/kyverno/readiness-checker:latest"
check testImageGlobalRegistry "registry.example.com/kyverno/readiness-checker:latest"

# webhooksCleanup.image falls back to Chart.AppVersion for its tag.
check webhooksCleanupImageDefaultRegistry "ghcr.io/kyverno/readiness-checker:${APP_VERSION}"
check webhooksCleanupImageGlobalRegistry "registry.example.com/kyverno/readiness-checker:${APP_VERSION}"

printf 'Image registry fallback helpers verified\n'
