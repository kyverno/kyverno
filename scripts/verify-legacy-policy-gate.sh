#!/usr/bin/env bash
#
# verify-legacy-policy-gate.sh
#
# Exercises the #17490 Helm legacy-policy gate
# (charts/kyverno/templates/validate-legacy-policies.yaml) against the
# current kubectl context, covering the scenarios from the design doc's
# risk register and test plan:
#
#   1. legacy CRDs absent                            -> install must PASS
#   2. legacy CRDs present, zero instances            -> install must PASS
#   3. legacy CRDs present, an instance exists         -> install must be BLOCKED,
#                                                          with the count, the
#                                                          offending name, and the
#                                                          opt-out hint in the error
#   4. legacy CRDs present, an instance exists, opt-out -> install must PASS
#
# It uses `helm install --dry-run=server`, which evaluates `lookup` against
# the live cluster without actually installing Kyverno, so this does not
# require Kyverno's images and can run against a bare kind cluster with only
# the Helm chart's CRDs applied. It requires a reachable Kubernetes API
# server (a kind cluster in CI) as the current kubectl context.
#
# SAFETY: this script deletes the five kyverno.io legacy CRDs (clusterpolicies,
# policies, cleanuppolicies, clustercleanuppolicies, policyexceptions) against
# whatever the CURRENT kubectl context is, both to set up scenario 1 and in its
# exit-trap cleanup. Deleting a CRD cascade-deletes every one of its CRs. On a
# real cluster running Kyverno, that means every ClusterPolicy, Policy,
# CleanupPolicy, ClusterCleanupPolicy, and PolicyException in the cluster -
# irreversible data loss. Before doing anything destructive, this script
# refuses to proceed unless (a) the current context's NAME starts with
# "kind-" AND (b) the cluster is verified LIVE to be a genuine kind cluster
# (every node reports a "kind://" spec.providerID) - a context name alone is
# not proof, since nothing stops a real cluster from being named
# "kind-production" - or the operator has explicitly opted in. See the
# guard below.
#
# Usage: scripts/verify-legacy-policy-gate.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${ROOT_DIR}/charts/kyverno"
RELEASE_NAME="legacy-policy-gate-verify"
NAMESPACE="legacy-policy-gate-verify"
# Honor a pinned helm binary passed in by the Makefile (HELM=$(HELM), the
# .tools/helm the rest of the build uses), falling back to whatever "helm"
# resolves to on PATH when run standalone.
HELM="${HELM:-helm}"
# The plain `helm template` call below (used only to extract the CRD YAML,
# not to exercise the gate) doesn't talk to a live cluster, so it falls back
# to Helm's own built-in default Capabilities.KubeVersion, which can be
# older than this chart's `kubeVersion` constraint. Pin it the same way the
# Makefile's other `helm template --kube-version $(KUBE_VERSION)` calls do.
# The `install()` function's `--dry-run=server` calls don't need this: they
# discover the real, live cluster version instead.
KUBE_VERSION="${KUBE_VERSION:-v1.25.0}"

CRD_NAMES=(
  clusterpolicies.kyverno.io
  policies.kyverno.io
  cleanuppolicies.kyverno.io
  clustercleanuppolicies.kyverno.io
  policyexceptions.kyverno.io
)

log() { echo "[verify-legacy-policy-gate] $*"; }
fail() { echo "[verify-legacy-policy-gate] FAIL: $*" >&2; exit 1; }

# --- destructive-action guard: must run before anything touches the cluster ---
#
# A context NAME starting with "kind-" is not proof of anything by itself -
# an operator could easily have a real cluster on a context literally named
# "kind-production", and this script would cascade-delete its CRDs. So,
# unless the operator explicitly overrides, this also verifies the CLUSTER
# ITSELF is a genuine kind cluster: every node's spec.providerID must use
# kind's "kind://docker/<cluster>/<node>" scheme.
CURRENT_CONTEXT="$(kubectl config current-context 2>/dev/null || true)"
if [ -z "${CURRENT_CONTEXT}" ]; then
  fail "could not determine the current kubectl context; refusing to run a script that deletes CRDs against an unknown cluster"
fi

# all_providerids_are_kind takes the space-separated list of every node's
# spec.providerID (as returned by `kubectl get nodes -o jsonpath=...`) and
# succeeds only if the list is non-empty and every entry starts with
# "kind://" - the scheme kind sets on every node it creates. An empty list
# (no nodes returned, or the kubectl call failed) fails CLOSED, not open.
# Factored out as a function so it can be exercised directly with a fake
# list, independent of a live cluster.
all_providerids_are_kind() {
  local providerids="$1"
  [ -n "${providerids}" ] || return 1
  local id
  for id in ${providerids}; do
    case "${id}" in
      kind://*) ;;
      *) return 1 ;;
    esac
  done
  return 0
}

guard_refuse() {
  cat >&2 <<MSG
[verify-legacy-policy-gate] FAIL: refusing to run against kubectl context '${CURRENT_CONTEXT}'.

This script deletes the five kyverno.io legacy CRDs (clusterpolicies,
policies, cleanuppolicies, clustercleanuppolicies, policyexceptions) to set up
its test scenarios, and again on exit as cleanup. Deleting a CRD deletes every
custom resource of that kind cluster-wide. If this context is a real cluster,
that is irreversible data loss for every legacy Kyverno policy on it.

$1

This only runs automatically against a context that both (a) has a name
starting with "kind-", AND (b) is verified live to be a genuine kind
cluster (every node's spec.providerID starts with "kind://"). A context
name alone is not proof - it could be a real cluster renamed to look
disposable. To run this anywhere else, you must explicitly acknowledge the
risk:

  VERIFY_ALLOW_DESTRUCTIVE=1 $0

Nothing has been touched.
MSG
  exit 1
}

if [ "${VERIFY_ALLOW_DESTRUCTIVE:-}" = "1" ]; then
  log "VERIFY_ALLOW_DESTRUCTIVE=1 was set explicitly; proceeding against kubectl context '${CURRENT_CONTEXT}' without verifying it is a genuine kind cluster"
else
  case "${CURRENT_CONTEXT}" in
    kind-*) ;;
    *) guard_refuse "Its name does not start with \"kind-\"." ;;
  esac
  NODE_PROVIDER_IDS="$(kubectl get nodes -o jsonpath='{.items[*].spec.providerID}' 2>/dev/null || true)"
  if ! all_providerids_are_kind "${NODE_PROVIDER_IDS}"; then
    guard_refuse "Its name starts with \"kind-\", but its nodes did not all verify live as genuine kind nodes (expected every node to report a kind://... spec.providerID; got: '${NODE_PROVIDER_IDS:-<empty or unreachable>}')."
  fi
  log "current kubectl context '${CURRENT_CONTEXT}' verified as a genuine kind cluster (name starts with kind-, and every node's providerID is kind://...), proceeding"
fi

WORK_DIR="$(mktemp -d)"

cleanup() {
  kubectl delete clusterpolicy "${RELEASE_NAME}" --ignore-not-found >/dev/null 2>&1 || true
  for name in "${CRD_NAMES[@]}"; do
    kubectl delete crd "${name}" --ignore-not-found >/dev/null 2>&1 || true
  done
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

# Install (helm dependency build needs the CRD subchart's templates, but the
# CRDs themselves are extracted from a plain `helm template` render, so this
# does not depend on a packaged/published subchart being available).
log "extracting the five legacy CRDs from a chart render"
"${HELM}" template "${RELEASE_NAME}" "${CHART_DIR}" --kube-version "${KUBE_VERSION}" --set upgrade.allowLegacyPolicies=true > "${WORK_DIR}/full-render.yaml"
for name in "${CRD_NAMES[@]}"; do
  awk -v n="name: ${name}" 'BEGIN{RS="---\n"} $0 ~ n {print; exit}' "${WORK_DIR}/full-render.yaml" > "${WORK_DIR}/${name}.yaml"
  if [ ! -s "${WORK_DIR}/${name}.yaml" ]; then
    fail "could not extract CRD ${name} from the chart render; is crds.install still true by default?"
  fi
done

install() {
  "${HELM}" install "${RELEASE_NAME}" "${CHART_DIR}" \
    --namespace "${NAMESPACE}" --create-namespace \
    --set crds.install=false \
    --dry-run=server \
    "$@"
}

log "scenario 1: legacy CRDs absent, expect PASS"
for name in "${CRD_NAMES[@]}"; do kubectl delete crd "${name}" --ignore-not-found >/dev/null 2>&1 || true; done
if ! install >"${WORK_DIR}/scenario1.log" 2>&1; then
  cat "${WORK_DIR}/scenario1.log" >&2
  fail "scenario 1 (CRDs absent) was expected to pass"
fi
log "PASS: scenario 1"

log "scenario 2: legacy CRDs present, zero instances, expect PASS"
for name in "${CRD_NAMES[@]}"; do kubectl create -f "${WORK_DIR}/${name}.yaml" >/dev/null; done
for name in "${CRD_NAMES[@]}"; do kubectl wait --for=condition=Established "crd/${name}" --timeout=30s >/dev/null; done
if ! install >"${WORK_DIR}/scenario2.log" 2>&1; then
  cat "${WORK_DIR}/scenario2.log" >&2
  fail "scenario 2 (CRDs present, zero instances) was expected to pass"
fi
log "PASS: scenario 2"

log "scenario 3: legacy CRDs present, an instance exists, expect BLOCK"
kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: ${RELEASE_NAME}
spec:
  rules: []
EOF
if install >"${WORK_DIR}/scenario3.log" 2>&1; then
  cat "${WORK_DIR}/scenario3.log" >&2
  fail "scenario 3 (legacy CR present) was expected to be blocked, but install succeeded"
fi
grep -q "ClusterPolicy: 1" "${WORK_DIR}/scenario3.log" \
  || { cat "${WORK_DIR}/scenario3.log" >&2; fail "scenario 3 error output is missing the per-kind count"; }
grep -q "${RELEASE_NAME}" "${WORK_DIR}/scenario3.log" \
  || { cat "${WORK_DIR}/scenario3.log" >&2; fail "scenario 3 error output is missing the offending resource name"; }
grep -q "upgrade.allowLegacyPolicies=true" "${WORK_DIR}/scenario3.log" \
  || { cat "${WORK_DIR}/scenario3.log" >&2; fail "scenario 3 error output is missing the opt-out hint"; }
log "PASS: scenario 3"

log "scenario 4: legacy CRDs present, an instance exists, opt-out set, expect PASS"
if ! install --set upgrade.allowLegacyPolicies=true >"${WORK_DIR}/scenario4.log" 2>&1; then
  cat "${WORK_DIR}/scenario4.log" >&2
  fail "scenario 4 (opt-out) was expected to pass"
fi
log "PASS: scenario 4"

log "all legacy-policy gate scenarios passed"
