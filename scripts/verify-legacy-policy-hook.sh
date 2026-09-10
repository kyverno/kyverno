#!/usr/bin/env bash
#
# verify-legacy-policy-hook.sh
#
# Exercises the #17490 Helm legacy-policy gate's SECOND layer: the
# pre-install,pre-upgrade hook Job
# (charts/kyverno/templates/hooks/pre-install-check-legacy-policies.yaml)
# that runs the hidden `kyverno check-legacy-policies` CLI command.
#
# This is deliberately separate from scripts/verify-legacy-policy-gate.sh,
# which covers the render-time `lookup`+`fail` gate via
# `helm install --dry-run=server`. That mode never executes Helm hooks, so
# it cannot exercise this Job's actual blocking behavior, and on a real
# `helm upgrade` the render-time gate fails first, so the hook never gets an
# independent run either. The only faithful way to test this Job is to
# render it on its own and apply it to a live cluster that already has the
# legacy CRDs registered and the `kyverno-cli` image loaded - i.e. AFTER
# Kyverno itself has been installed with the locally built images (see
# `make kind-install-kyverno`).
#
# It covers:
#   A. a legacy CR exists    -> the Job must FAIL, with a pod that terminated
#                                non-zero, and logs containing the per-kind
#                                count, the offending resource name, and the
#                                migration guidance
#   B. no legacy CR exists   -> the Job must SUCCEED, with logs reporting no
#                                legacy resources found
#   C. a legacy CR exists, upgrade.allowLegacyPolicies=true (opt-out)
#                             -> the hook Job (and its ServiceAccount/RBAC)
#                                must not be rendered at all, so the install
#                                proceeds
#   D. upgrade.legacyPolicyCheck.enabled=false (hook disabled, no opt-out)
#                             -> likewise not rendered
#
# SAFETY: like verify-legacy-policy-gate.sh, this script creates a
# cluster-scoped legacy ClusterPolicy (and deletes it) and applies/deletes
# cluster-scoped RBAC (ClusterRole/ClusterRoleBinding) for the hook's
# ServiceAccount. It refuses to run unless (a) the current context's NAME
# starts with "kind-" AND (b) the cluster is verified LIVE to be a genuine
# kind cluster (every node reports a "kind://" spec.providerID) - a context
# name alone is not proof, since nothing stops a real cluster from being
# named "kind-production" - or the operator has explicitly opted in. See
# the guard below.
#
# Usage: scripts/verify-legacy-policy-hook.sh
#
# Expects (set by the `verify-legacy-policy-hook` Makefile target, with
# fallbacks below for standalone use):
#   HELM             - pinned helm binary
#   KUBE_VERSION     - kube version to pin the template render to
#   LOCAL_REGISTRY   - registry the local kyverno-cli image was pushed/loaded to
#   LOCAL_CLI_REPO   - repository of the local kyverno-cli image
#   GIT_SHA          - tag of the local kyverno-cli image (the image `make
#                       kind-load-cli`/`kind-install-kyverno` loads into kind)

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${ROOT_DIR}/charts/kyverno"
# The rendered resource names (kyverno:check-legacy-policies ClusterRole/
# ClusterRoleBinding, kyverno-check-legacy-policies ServiceAccount/Job) come
# from `kyverno.fullname`, which is just the release name when it equals
# the chart name. Render as release "kyverno" so those names match the
# fixed names this script's cleanup deletes, regardless of what the actual
# installed release happens to be called.
RELEASE_NAME="kyverno"
NAMESPACE="kyverno"
# Give the test ClusterPolicy a run-unique name so this script can never
# delete a real ClusterPolicy that happens to share a fixed name. (The hook
# RBAC/Job/ServiceAccount names below are the chart's own hook resource names,
# derived from the render release name, not user data, so they stay fixed.)
CR_NAME="legacy-policy-hook-verify-$$-${RANDOM}"
JOB_NAME="kyverno-check-legacy-policies"

HELM="${HELM:-helm}"
KUBE_VERSION="${KUBE_VERSION:-v1.25.0}"
LOCAL_REGISTRY="${LOCAL_REGISTRY:-ghcr.io}"
LOCAL_CLI_REPO="${LOCAL_CLI_REPO:-kyverno/kyverno-cli}"
GIT_SHA="${GIT_SHA:-$(git -C "${ROOT_DIR}" rev-parse HEAD)}"

log() { echo "[verify-legacy-policy-hook] $*"; }
fail() { echo "[verify-legacy-policy-hook] FAIL: $*" >&2; exit 1; }

# --- destructive-action guard: must run before anything touches the cluster ---
#
# A context NAME starting with "kind-" is not proof of anything by itself -
# an operator could easily have a real cluster on a context literally named
# "kind-production", and this script would create/delete cluster-scoped
# resources against it. So, unless the operator explicitly overrides, this
# also verifies the CLUSTER ITSELF is a genuine kind cluster: every node's
# spec.providerID must use kind's "kind://docker/<cluster>/<node>" scheme.
CURRENT_CONTEXT="$(kubectl config current-context 2>/dev/null || true)"
if [ -z "${CURRENT_CONTEXT}" ]; then
  fail "could not determine the current kubectl context; refusing to run a script that creates cluster-scoped resources against an unknown cluster"
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
[verify-legacy-policy-hook] FAIL: refusing to run against kubectl context '${CURRENT_CONTEXT}'.

This script creates a cluster-scoped legacy ClusterPolicy and cluster-scoped
RBAC (ClusterRole/ClusterRoleBinding) to exercise the legacy-policy check
hook Job, and deletes them again on exit.

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

# Delete the cluster-scoped resources this script owns, without touching
# WORK_DIR (created below). Idempotent: safe to call both up front, in case
# a previous interrupted run left these behind, and on exit.
cleanup_resources() {
  kubectl delete job "${JOB_NAME}" --namespace "${NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete clusterrolebinding "kyverno:check-legacy-policies" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete clusterrole "kyverno:check-legacy-policies" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete serviceaccount "kyverno-check-legacy-policies" --namespace "${NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete clusterpolicy "${CR_NAME}" --ignore-not-found >/dev/null 2>&1 || true
}

# Make sure we're starting clean (in case a previous run was interrupted).
cleanup_resources

# This script must run after Kyverno (and its CRDs) is installed: with the
# legacy CRDs absent, `check-legacy-policies` treats every kind as zero
# instances (see countKind's CRD-not-found handling) and scenario B would
# pass for the wrong reason, silently. Fail loudly up front instead.
if ! kubectl get crd clusterpolicies.kyverno.io >/dev/null 2>&1; then
  fail "the clusterpolicies.kyverno.io CRD is not registered on this cluster; this script must run AFTER Kyverno is installed (e.g. after 'make kind-install-kyverno'), so the legacy CRDs it needs are present"
fi

WORK_DIR="$(mktemp -d)"

cleanup() {
  cleanup_resources
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

render_hook() {
  # Render just the hook Job template with the SAME image overrides
  # `kind-install-kyverno` uses, so the rendered image is the one already
  # loaded into the kind node. `--show-only` renders the whole file
  # (ClusterRole + ClusterRoleBinding + ServiceAccount + Job, in that
  # order), which is fine: we want to apply all of it.
  "${HELM}" template "${RELEASE_NAME}" "${CHART_DIR}" \
    --kube-version "${KUBE_VERSION}" \
    --namespace "${NAMESPACE}" \
    --show-only templates/hooks/pre-install-check-legacy-policies.yaml \
    --set upgrade.legacyPolicyCheck.image.registry="${LOCAL_REGISTRY}" \
    --set upgrade.legacyPolicyCheck.image.repository="${LOCAL_CLI_REPO}" \
    --set upgrade.legacyPolicyCheck.image.tag="${GIT_SHA}"
}

apply_hook_and_wait() {
  # We `kubectl apply` this render directly rather than going through a real
  # Helm install/upgrade, so Helm's hook-delete-policy annotations
  # (helm.sh/hook-delete-policy) are never interpreted by anything here -
  # they're inert metadata to a plain `kubectl apply`, and no live Helm
  # release is managing this Job's lifecycle. So there's nothing that would
  # race-delete the Job out from under us before we can inspect it; we don't
  # need to strip the annotations. (They stay in the render so what's
  # applied matches what a real Helm hook execution would apply.)
  local out_file="$1"
  render_hook > "${out_file}"
  kubectl delete job "${JOB_NAME}" --namespace "${NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl apply -f "${out_file}" >/dev/null

  # Wait for the Job to reach a terminal state, whichever comes first.
  # `kubectl wait --for=condition=X` blocks until X specifically becomes
  # true (or the timeout), so waiting for Complete first would waste the
  # full timeout on a Job that is actually going to Fail. Poll for either
  # condition instead.
  local waited=0
  local timeout=90
  while [ "${waited}" -lt "${timeout}" ]; do
    local complete failed
    complete="$(job_condition Complete)"
    failed="$(job_condition Failed)"
    if [ "${complete}" = "True" ] || [ "${failed}" = "True" ]; then
      return 0
    fi
    sleep 2
    waited=$((waited + 2))
  done
  return 0
}

job_logs() {
  kubectl logs "job/${JOB_NAME}" --namespace "${NAMESPACE}" --all-containers 2>&1 || true
}

job_condition() {
  local type="$1"
  kubectl get job "${JOB_NAME}" --namespace "${NAMESPACE}" \
    -o jsonpath="{.status.conditions[?(@.type==\"${type}\")].status}" 2>/dev/null || true
}

pod_exit_code() {
  kubectl get pods --namespace "${NAMESPACE}" -l "job-name=${JOB_NAME}" \
    -o jsonpath='{.items[0].status.containerStatuses[0].state.terminated.exitCode}' 2>/dev/null || true
}

# assert_hook_absent renders the WHOLE chart (not --show-only) with the
# given extra --set args, plus the same image overrides render_hook uses,
# and asserts no check-legacy-policies resource (Job/ServiceAccount/
# ClusterRole/ClusterRoleBinding) appears anywhere in the render.
#
# This deliberately does not use `helm template --show-only` here: when the
# targeted file renders to nothing (as this one does under the opt-out or
# with the hook disabled), `--show-only`'s behavior on an empty match is not
# consistent across Helm versions - it can error out ("could not find
# template") or print nothing, and either way that ambiguity would be hard
# to tell apart from a genuine template bug. Rendering the whole chart never
# has that ambiguity (a real syntax error still fails the `helm template`
# call itself, so this can't mask one), and then grepping for the hook's
# resource names cleanly proves the hook is absent either way.
assert_hook_absent() {
  local label="$1"
  shift
  local out_file="${WORK_DIR}/${label}.yaml"
  "${HELM}" template "${RELEASE_NAME}" "${CHART_DIR}" \
    --kube-version "${KUBE_VERSION}" \
    --namespace "${NAMESPACE}" \
    --set upgrade.legacyPolicyCheck.image.registry="${LOCAL_REGISTRY}" \
    --set upgrade.legacyPolicyCheck.image.repository="${LOCAL_CLI_REPO}" \
    --set upgrade.legacyPolicyCheck.image.tag="${GIT_SHA}" \
    "$@" > "${out_file}"
  if grep -q "check-legacy-policies" "${out_file}"; then
    grep -n "check-legacy-policies" "${out_file}" >&2
    fail "${label}: expected no check-legacy-policies resources in the render, but found some (see above)"
  fi
}

log "scenario A: legacy CR exists, expect the hook Job to FAIL"
kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: ${CR_NAME}
spec:
  rules: []
EOF

apply_hook_and_wait "${WORK_DIR}/hook-blocked.yaml"

FAILED_STATUS="$(job_condition Failed)"
COMPLETE_STATUS="$(job_condition Complete)"
if [ "${COMPLETE_STATUS}" = "True" ]; then
  job_logs >&2
  fail "scenario A: the hook Job unexpectedly SUCCEEDED while a legacy ClusterPolicy exists"
fi
if [ "${FAILED_STATUS}" != "True" ]; then
  job_logs >&2
  fail "scenario A: the hook Job did not reach a Failed condition within the timeout (status: Failed=${FAILED_STATUS}, Complete=${COMPLETE_STATUS})"
fi

EXIT_CODE="$(pod_exit_code)"
if [ -z "${EXIT_CODE}" ] || [ "${EXIT_CODE}" = "0" ]; then
  job_logs >&2
  fail "scenario A: expected the Job's pod container to have terminated with a non-zero exit code, got '${EXIT_CODE}'"
fi

LOGS="$(job_logs)"
echo "${LOGS}" | grep -q "ClusterPolicy: 1" \
  || { echo "${LOGS}" >&2; fail "scenario A: pod logs are missing the per-kind count"; }
echo "${LOGS}" | grep -q "${CR_NAME}" \
  || { echo "${LOGS}" >&2; fail "scenario A: pod logs are missing the offending resource name"; }
echo "${LOGS}" | grep -qi "migrate" \
  || { echo "${LOGS}" >&2; fail "scenario A: pod logs are missing migration guidance"; }
echo "${LOGS}" | grep -q "upgrade.allowLegacyPolicies=true" \
  || { echo "${LOGS}" >&2; fail "scenario A: pod logs are missing the opt-out hint"; }
log "PASS: scenario A (Job failed, exit code ${EXIT_CODE}, logs contain count/name/migration guidance)"

log "scenario B: no legacy CR present, expect the hook Job to SUCCEED"
kubectl delete clusterpolicy "${CR_NAME}" --ignore-not-found >/dev/null

apply_hook_and_wait "${WORK_DIR}/hook-pass.yaml"

FAILED_STATUS="$(job_condition Failed)"
COMPLETE_STATUS="$(job_condition Complete)"
if [ "${FAILED_STATUS}" = "True" ]; then
  job_logs >&2
  fail "scenario B: the hook Job unexpectedly FAILED with no legacy resources present"
fi
if [ "${COMPLETE_STATUS}" != "True" ]; then
  job_logs >&2
  fail "scenario B: the hook Job did not reach a Complete condition within the timeout (status: Failed=${FAILED_STATUS}, Complete=${COMPLETE_STATUS})"
fi

EXIT_CODE="$(pod_exit_code)"
if [ "${EXIT_CODE}" != "0" ]; then
  job_logs >&2
  fail "scenario B: expected the Job's pod container to have exited 0, got '${EXIT_CODE}'"
fi

LOGS="$(job_logs)"
echo "${LOGS}" | grep -q "no legacy policy resources found" \
  || { echo "${LOGS}" >&2; fail "scenario B: pod logs do not report a clean result"; }
log "PASS: scenario B (Job succeeded, exit code 0, logs report no legacy resources)"

log "scenario C: legacy CR exists, opt-out set, expect hook Job NOT rendered"
kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: ${CR_NAME}
spec:
  rules: []
EOF
assert_hook_absent "hook-optout" --set upgrade.allowLegacyPolicies=true
log "PASS: scenario C (opt-out set with a legacy CR present -> hook Job not rendered)"

log "scenario D: legacyPolicyCheck.enabled=false, expect hook Job NOT rendered"
assert_hook_absent "hook-disabled" --set upgrade.legacyPolicyCheck.enabled=false
log "PASS: scenario D (legacyPolicyCheck.enabled=false -> hook Job not rendered)"

kubectl delete clusterpolicy "${CR_NAME}" --ignore-not-found >/dev/null

log "all legacy-policy hook Job scenarios passed"
