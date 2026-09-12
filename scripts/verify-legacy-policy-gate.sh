#!/usr/bin/env bash
#
# verify-legacy-policy-gate.sh
#
# Exercises the #17490 Helm legacy-policy gate
# (charts/kyverno/templates/validate-legacy-policies.yaml) against the
# current kubectl context, covering the scenarios from the design doc's
# risk register and test plan:
#
#   1. legacy CRDs absent                              -> install must PASS
#   2. legacy CRDs present, zero instances              -> install must PASS
#   3. legacy CRDs present, an instance of EACH of the
#      five legacy kinds exists (ClusterPolicy, Policy,
#      CleanupPolicy, ClusterCleanupPolicy,
#      PolicyException)                                 -> install must be
#                                                          BLOCKED, with the
#                                                          count, the offending
#                                                          name, and the
#                                                          migration guidance
#                                                          for EACH kind, plus
#                                                          the opt-out hint,
#                                                          in the error
#   4. same five instances still present, opt-out set   -> install must PASS
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

# Scenario 3/4 create one instance of EACH of the five legacy kinds, so a
# regression that only detects ClusterPolicy (and silently stops counting
# Policy/CleanupPolicy/ClusterCleanupPolicy/PolicyException) would still
# fail CI. Every fixture name is run-unique (a fixed PID+RANDOM suffix), so
# this script's cleanup can never delete a real resource that happens to
# share a fixed name, and a concurrent run of this same script can't
# collide with this one. The namespaced kinds (Policy, CleanupPolicy,
# PolicyException) live in a dedicated, also run-unique, namespace.
RUN_ID="$$-${RANDOM}"
FIXTURE_NAMESPACE="legacy-policy-gate-verify-fixtures-${RUN_ID}"
CLUSTERPOLICY_NAME="legacy-policy-gate-verify-clusterpolicy-${RUN_ID}"
POLICY_NAME="legacy-policy-gate-verify-policy-${RUN_ID}"
CLEANUPPOLICY_NAME="legacy-policy-gate-verify-cleanuppolicy-${RUN_ID}"
CLUSTERCLEANUPPOLICY_NAME="legacy-policy-gate-verify-clustercleanuppolicy-${RUN_ID}"
POLICYEXCEPTION_NAME="legacy-policy-gate-verify-policyexception-${RUN_ID}"

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
  # Delete this run's own fixture of each of the five legacy kinds by its
  # run-unique name, fully group-qualified (e.g. "policyexceptions.kyverno.io"
  # rather than bare "policyexception") since the newer policies.kyverno.io
  # API group also has a PolicyException kind - an unqualified `kubectl
  # delete policyexception` would be ambiguous once both CRDs are
  # registered, as they are on a fully installed Kyverno.
  kubectl delete clusterpolicies.kyverno.io "${CLUSTERPOLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUPPOLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete policies.kyverno.io "${POLICY_NAME}" --namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete cleanuppolicies.kyverno.io "${CLEANUPPOLICY_NAME}" --namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete policyexceptions.kyverno.io "${POLICYEXCEPTION_NAME}" --namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
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

log "scenario 3: legacy CRDs present, an instance of each of the five legacy kinds exists, expect BLOCK"
kubectl create namespace "${FIXTURE_NAMESPACE}" >/dev/null

kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: ${CLUSTERPOLICY_NAME}
spec:
  rules: []
EOF

kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v1
kind: Policy
metadata:
  name: ${POLICY_NAME}
  namespace: ${FIXTURE_NAMESPACE}
spec:
  rules: []
EOF

# CleanupPolicy/ClusterCleanupPolicy (kyverno.io/v2) require a schedule and
# a match block; ClusterPolicy/Policy above don't require a rule to exist.
# Shape sourced from the deprecation conformance fixtures added alongside
# this chart's gate (test/conformance/chainsaw/deprecations/create-blocked/
# cleanup-policy-v2.yaml and cluster-cleanup-policy-v2.yaml), which are
# already proven schema-valid. This script never installs live Kyverno (only
# the five bare CRDs), so unlike verify-legacy-policy-hook.sh there is no
# live cleanup-controller admission webhook here to validate these specs'
# RBAC - only the CRD's OpenAPI schema applies.
kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v2
kind: CleanupPolicy
metadata:
  name: ${CLEANUPPOLICY_NAME}
  namespace: ${FIXTURE_NAMESPACE}
spec:
  schedule: "0 0 * * *"
  match:
    any:
    - resources:
        kinds:
        - Pod
        names:
        - legacy-policy-gate-verify-does-not-exist
EOF

kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v2
kind: ClusterCleanupPolicy
metadata:
  name: ${CLUSTERCLEANUPPOLICY_NAME}
spec:
  schedule: "0 0 * * *"
  match:
    any:
    - resources:
        kinds:
        - Namespace
        names:
        - legacy-policy-gate-verify-does-not-exist
EOF

# PolicyException (kyverno.io/v2) shape sourced from the same conformance
# fixtures (policy-exception-v2.yaml): it requires at least one exception
# entry and a match block.
kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v2
kind: PolicyException
metadata:
  name: ${POLICYEXCEPTION_NAME}
  namespace: ${FIXTURE_NAMESPACE}
spec:
  exceptions:
  - policyName: does-not-exist
    ruleNames:
    - "*"
  match:
    any:
    - resources:
        kinds:
        - Pod
EOF

if install >"${WORK_DIR}/scenario3.log" 2>&1; then
  cat "${WORK_DIR}/scenario3.log" >&2
  fail "scenario 3 (legacy CRs present) was expected to be blocked, but install succeeded"
fi

# assert_kind_reported checks the per-kind count line and the offending
# resource name for one kind. The count check is anchored on "- <Kind>: 1"
# (matching the render's "  | - <Kind>: <count>" line format) rather than a
# bare "<Kind>: 1", because several of these kind names are suffixes of each
# other (e.g. "ClusterPolicy: 1" and "CleanupPolicy: 1" both literally end
# in "Policy: 1") - an unanchored check for "Policy: 1" would spuriously
# pass off of ClusterPolicy's or CleanupPolicy's line even if Policy's own
# line were missing, silently defeating the point of checking each kind
# individually. The "- " prefix immediately before the kind name in the
# real line format is what makes each of these five patterns mutually
# exclusive substrings of one another.
assert_kind_reported() {
  local kind="$1" name="$2"
  grep -q -- "- ${kind}: 1" "${WORK_DIR}/scenario3.log" \
    || { cat "${WORK_DIR}/scenario3.log" >&2; fail "scenario 3 error output is missing the per-kind count for ${kind}"; }
  grep -q -- "${name}" "${WORK_DIR}/scenario3.log" \
    || { cat "${WORK_DIR}/scenario3.log" >&2; fail "scenario 3 error output is missing the offending resource name for ${kind} (${name})"; }
}
assert_kind_reported "ClusterPolicy" "${CLUSTERPOLICY_NAME}"
assert_kind_reported "Policy" "${FIXTURE_NAMESPACE}/${POLICY_NAME}"
assert_kind_reported "CleanupPolicy" "${FIXTURE_NAMESPACE}/${CLEANUPPOLICY_NAME}"
assert_kind_reported "ClusterCleanupPolicy" "${CLUSTERCLEANUPPOLICY_NAME}"
assert_kind_reported "PolicyException" "${FIXTURE_NAMESPACE}/${POLICYEXCEPTION_NAME}"
grep -q "https://kyverno.io/docs/guides/migration-to-cel/" "${WORK_DIR}/scenario3.log" \
  || { cat "${WORK_DIR}/scenario3.log" >&2; fail "scenario 3 error output is missing the migration guidance"; }
grep -q "upgrade.allowLegacyPolicies=true" "${WORK_DIR}/scenario3.log" \
  || { cat "${WORK_DIR}/scenario3.log" >&2; fail "scenario 3 error output is missing the opt-out hint"; }
log "PASS: scenario 3"

log "scenario 4: same five legacy instances still present, opt-out set, expect PASS"
if ! install --set upgrade.allowLegacyPolicies=true >"${WORK_DIR}/scenario4.log" 2>&1; then
  cat "${WORK_DIR}/scenario4.log" >&2
  fail "scenario 4 (opt-out) was expected to pass"
fi
log "PASS: scenario 4"

log "all legacy-policy gate scenarios passed"
