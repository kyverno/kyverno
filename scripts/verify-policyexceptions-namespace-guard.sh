#!/usr/bin/env bash
#
# verify-policyexceptions-namespace-guard.sh
#
# Asserts that `helm template` FAILS when policyExceptions.enabled=true and
# the effective exceptionNamespace is empty, for every place that combination
# can be set: the top-level `features.policyExceptions` block, and every
# controller whose deployment template actually wires the policyExceptions
# flags into its featuresOverride `pick` list — admissionController,
# backgroundController, and reportsController. cleanupController's template
# never picks "policyExceptions" (the cleanup controller doesn't evaluate
# PolicyExceptions), so it has no guard to verify and is deliberately not
# checked here.
#
# Why this exists: an admission/background/reports controller that starts
# with --enablePolicyException=true and no --exceptionNamespace value admits
# PolicyException resources but then silently ignores them at evaluation
# time (see pkg/cel/engine/exception.go, ExceptionScopeNone) — the chart
# should refuse to render that configuration rather than deploy it and have
# the controller log a startup error or emit a webhook warning about it at
# runtime. The chart's `kyverno.features.flags` helper in _helpers.tpl is
# the guard against deploying that silently-inert configuration. This
# script is the regression test for that guard: if a future edit to
# _helpers.tpl narrows the `fail` condition, drops it from one of these
# controllers' render paths, or the install/upgrade otherwise succeeds with
# this combination, this script catches it with no cluster required.
#
# Usage: scripts/verify-policyexceptions-namespace-guard.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${ROOT_DIR}/charts/kyverno"
RELEASE_NAME="polex-namespace-guard-verify"
# Honor a pinned helm binary passed in by the Makefile (HELM=$(HELM), the
# .tools/helm the rest of the build uses), falling back to whatever "helm"
# resolves to on PATH when run standalone.
HELM="${HELM:-helm}"
KUBE_VERSION="${KUBE_VERSION:-v1.25.0}"
EXPECTED_MESSAGE="policyExceptions.namespace must be set when policyExceptions.enabled is true"

log() { echo "[verify-policyexceptions-namespace-guard] $*"; }
fail() { echo "[verify-policyexceptions-namespace-guard] FAIL: $*" >&2; exit 1; }

# assert_render_fails NAME SET_ARGS...
# Renders the chart with the given --set arguments and asserts the render
# fails with the expected guard message. A render that succeeds, or fails
# for a different reason, is a failure of this check.
assert_render_fails() {
  local name="$1"
  shift
  local output
  local status=0
  output="$("${HELM}" template "${RELEASE_NAME}" "${CHART_DIR}" \
    --kube-version "${KUBE_VERSION}" \
    "$@" 2>&1)" || status=$?

  if [[ "${status}" -eq 0 ]]; then
    fail "${name}: expected \`helm template\` to fail, but it rendered successfully"
  fi
  if [[ "${output}" != *"${EXPECTED_MESSAGE}"* ]]; then
    fail "${name}: render failed as expected, but not with the guard message. Got:
${output}"
  fi
  log "${name}: OK (render failed with the expected guard message)"
}

# assert_render_succeeds NAME SET_ARGS...
# Companion positive check: the same combination with a namespace set must
# render cleanly, so the guard is proven to trigger on the empty-namespace
# condition specifically, not on policyExceptions.enabled alone.
assert_render_succeeds() {
  local name="$1"
  shift
  if ! "${HELM}" template "${RELEASE_NAME}" "${CHART_DIR}" \
    --kube-version "${KUBE_VERSION}" \
    "$@" > /dev/null 2>"${WORK_DIR}/stderr"; then
    fail "${name}: expected \`helm template\` to succeed once a namespace is set, but it failed:
$(cat "${WORK_DIR}/stderr")"
  fi
  log "${name}: OK (render succeeded once a namespace is set)"
}

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

log "rendering ${CHART_DIR} with --kube-version ${KUBE_VERSION}"

# 1. Global features.policyExceptions, no namespace: must fail.
assert_render_fails "global features.policyExceptions, empty namespace" \
  --set features.policyExceptions.enabled=true

# 2. Global features.policyExceptions, namespace set: must succeed.
assert_render_succeeds "global features.policyExceptions, namespace set" \
  --set features.policyExceptions.enabled=true \
  --set features.policyExceptions.namespace=kyverno

# 3. Per-controller featuresOverride, one controller at a time, no
#    namespace: each must fail even though the global block is unset.
# 4. Same featuresOverride path, namespace set: each must succeed.
for controller in admissionController backgroundController reportsController; do
  assert_render_fails "${controller}.featuresOverride.policyExceptions, empty namespace" \
    --set "${controller}.featuresOverride.policyExceptions.enabled=true"

  assert_render_succeeds "${controller}.featuresOverride.policyExceptions, namespace set" \
    --set "${controller}.featuresOverride.policyExceptions.enabled=true" \
    --set "${controller}.featuresOverride.policyExceptions.namespace=kyverno"
done

log "all checks passed"
