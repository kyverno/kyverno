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
#   A. an instance of EACH of the five legacy kinds exists (ClusterPolicy,
#      Policy, CleanupPolicy, ClusterCleanupPolicy, PolicyException) -> the
#      Job must FAIL, with a pod that terminated non-zero, and logs
#      containing, for EACH kind, its per-kind count and the offending
#      resource name, plus the migration guidance
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
# The rendered resource names (ClusterRole/ClusterRoleBinding/ServiceAccount/
# Job) come from `kyverno.fullname`, which is derived from the Helm RELEASE
# NAME. A real `helm install/upgrade kyverno` release renders these same
# templates with release name "kyverno", producing the FIXED names
# "kyverno-check-legacy-policies" (Job/ServiceAccount) and
# "kyverno:check-legacy-policies" (ClusterRole/ClusterRoleBinding). If this
# script rendered under that same "kyverno" release name, its own
# cleanup could delete a real, concurrently-running hook's resources instead
# of only its own - so it renders under a RUN-SPECIFIC release name instead,
# making every rendered resource name unique to this run, and never deletes
# by the chart's fixed hook-resource names. All cleanup/wait/log calls below
# derive the Job's name from what was actually rendered this run, not from
# any hardcoded name.
RUN_ID="$$-${RANDOM}"
RELEASE_NAME="legacy-hook-verify-${RUN_ID}"
NAMESPACE="kyverno"
# Run-unique names for the test fixtures, so this script can never delete a
# real resource that happens to share a fixed name. Scenario A creates one
# instance of EACH of the five legacy kinds (so a regression that only
# detects ClusterPolicy would still fail CI); CR_NAME (ClusterPolicy) is
# also reused by scenarios B/C/D, which only need one legacy CR present or
# absent. The namespaced kinds (Policy, CleanupPolicy, PolicyException)
# live in a dedicated, also run-unique, namespace - not the real "kyverno"
# namespace above, which is the hook's own install namespace.
CR_NAME="legacy-policy-hook-verify-${RUN_ID}"
FIXTURE_NAMESPACE="legacy-policy-hook-verify-fixtures-${RUN_ID}"
POLICY_NAME="legacy-policy-hook-verify-policy-${RUN_ID}"
CLEANUPPOLICY_NAME="legacy-policy-hook-verify-cleanuppolicy-${RUN_ID}"
CLUSTERCLEANUPPOLICY_NAME="legacy-policy-hook-verify-clustercleanuppolicy-${RUN_ID}"
POLICYEXCEPTION_NAME="legacy-policy-hook-verify-policyexception-${RUN_ID}"
# Populated once the hook is first rendered (see apply_hook_and_wait): the
# actual rendered Job name for this run, and the manifest file that was
# applied to the cluster (used by cleanup() to delete exactly what this run
# created, via `kubectl delete -f`, instead of any fixed resource name).
JOB_NAME=""
HOOK_MANIFEST=""

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

# With a run-unique RELEASE_NAME/CR_NAME (see above), no fixed-name
# resources from a *previous* run of this script could exist to pre-clean:
# every run's resources are named uniquely to that run, so there is nothing
# to collide with. The exit trap below handles this run's own cleanup.

# This script must run after Kyverno (and its CRDs) is installed: with the
# legacy CRDs absent, `check-legacy-policies` treats every kind as zero
# instances (see countKind's CRD-not-found handling) and scenario B would
# pass for the wrong reason, silently. Fail loudly up front instead.
if ! kubectl get crd clusterpolicies.kyverno.io >/dev/null 2>&1; then
  fail "the clusterpolicies.kyverno.io CRD is not registered on this cluster; this script must run AFTER Kyverno is installed (e.g. after 'make kind-install-kyverno'), so the legacy CRDs it needs are present"
fi

WORK_DIR="$(mktemp -d)"

# Deletes exactly the resources this run applied to the cluster - the hook
# Job/ServiceAccount/ClusterRole/ClusterRoleBinding via the last-applied
# rendered manifest (`kubectl delete -f`, so it can never touch a
# differently-named resource, fixed or otherwise), plus this run's own
# ClusterPolicy fixture. Guarded so it's safe to fire before the manifest
# has been rendered (e.g. the script fails before scenario A even applies
# anything) and safe to call more than once.
cleanup() {
  if [ -n "${HOOK_MANIFEST}" ] && [ -f "${HOOK_MANIFEST}" ]; then
    kubectl delete -f "${HOOK_MANIFEST}" --ignore-not-found >/dev/null 2>&1 || true
  fi
  # Delete this run's own fixture of each of the five legacy kinds by its
  # run-unique name, fully group-qualified (e.g. "policyexceptions.kyverno.io"
  # rather than bare "policyexception") since the newer policies.kyverno.io
  # API group also has a PolicyException kind - an unqualified `kubectl
  # delete policyexception` would be ambiguous on this fully installed
  # Kyverno, where both CRDs are registered.
  kubectl delete clusterpolicies.kyverno.io "${CR_NAME}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUPPOLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete policies.kyverno.io "${POLICY_NAME}" --namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete cleanuppolicies.kyverno.io "${CLEANUPPOLICY_NAME}" --namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete policyexceptions.kyverno.io "${POLICYEXCEPTION_NAME}" --namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
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

# job_name_from_manifest extracts the Job's metadata.name from a rendered
# multi-document hook manifest, without assuming any fixed name: it tracks
# the `kind:` of the current `---`-separated document and prints the first
# top-level (2-space-indented) `name:` it sees once that document's kind is
# "Job".
job_name_from_manifest() {
  awk '
    /^---/ { kind="" }
    /^kind: / { kind=$2 }
    kind == "Job" && /^  name: / { print $2; exit }
  ' "$1"
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
  HOOK_MANIFEST="${out_file}"
  if [ -z "${JOB_NAME}" ]; then
    JOB_NAME="$(job_name_from_manifest "${out_file}")"
    if [ -z "${JOB_NAME}" ]; then
      fail "could not determine the rendered hook Job's name from ${out_file}; the template's output shape may have changed"
    fi
    log "this run's rendered hook Job name: ${JOB_NAME} (release ${RELEASE_NAME})"
  fi
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

log "scenario A: an instance of each of the five legacy kinds exists, expect the hook Job to FAIL"
kubectl create namespace "${FIXTURE_NAMESPACE}" >/dev/null

kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: ${CR_NAME}
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
# a match block. Shape sourced from the deprecation conformance fixtures
# alongside this chart's gate
# (test/conformance/chainsaw/deprecations/create-blocked/cleanup-policy-v2.yaml
# and cluster-cleanup-policy-v2.yaml), EXCEPT ClusterCleanupPolicy below
# targets Pod rather than that fixture's Namespace: this script runs against
# a live cleanup-controller (unlike verify-legacy-policy-gate.sh, which
# never installs Kyverno), and that fixture is only ever exercised with the
# write-time block ON - it's asserting the create itself gets rejected, so
# it never reaches the cleanup-controller's own RBAC-validating webhook.
# Here, with the block OFF (migration-grace mode), that webhook DOES run,
# and the cleanup-controller's default ClusterRole only grants it delete on
# pods, not namespaces - so a Namespace-targeting policy would be rejected
# for an unrelated reason (insufficient RBAC), not proving anything about
# legacy-policy detection.
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
        - legacy-policy-hook-verify-does-not-exist
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
        - Pod
        names:
        - legacy-policy-hook-verify-does-not-exist
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

# assert_kind_logged checks the per-kind count line and the offending
# resource name for one kind in the Job's pod logs. The count check is
# anchored on "- <Kind>: 1" (matching the CLI's "  - <Kind>: <count>" line
# format) rather than a bare "<Kind>: 1", because several of these kind
# names are suffixes of each other (e.g. "ClusterPolicy: 1" and
# "CleanupPolicy: 1" both literally end in "Policy: 1") - an unanchored
# check for "Policy: 1" would spuriously pass off of ClusterPolicy's or
# CleanupPolicy's line even if Policy's own line were missing, silently
# defeating the point of checking each kind individually. The "- " prefix
# immediately before the kind name in the real line format is what makes
# each of these five patterns mutually exclusive substrings of one another.
assert_kind_logged() {
  local kind="$1" name="$2"
  echo "${LOGS}" | grep -q -- "- ${kind}: 1" \
    || { echo "${LOGS}" >&2; fail "scenario A: pod logs are missing the per-kind count for ${kind}"; }
  echo "${LOGS}" | grep -q -- "${name}" \
    || { echo "${LOGS}" >&2; fail "scenario A: pod logs are missing the offending resource name for ${kind} (${name})"; }
}
assert_kind_logged "ClusterPolicy" "${CR_NAME}"
assert_kind_logged "Policy" "${FIXTURE_NAMESPACE}/${POLICY_NAME}"
assert_kind_logged "CleanupPolicy" "${FIXTURE_NAMESPACE}/${CLEANUPPOLICY_NAME}"
assert_kind_logged "ClusterCleanupPolicy" "${CLUSTERCLEANUPPOLICY_NAME}"
assert_kind_logged "PolicyException" "${FIXTURE_NAMESPACE}/${POLICYEXCEPTION_NAME}"
echo "${LOGS}" | grep -qi "migrate" \
  || { echo "${LOGS}" >&2; fail "scenario A: pod logs are missing migration guidance"; }
echo "${LOGS}" | grep -q "upgrade.allowLegacyPolicies=true" \
  || { echo "${LOGS}" >&2; fail "scenario A: pod logs are missing the opt-out hint"; }
log "PASS: scenario A (Job failed, exit code ${EXIT_CODE}, logs contain count/name/migration guidance for all five kinds)"

log "scenario B: no legacy CR present, expect the hook Job to SUCCEED"
# Delete every one of scenario A's five fixtures, not just the
# ClusterPolicy - if any of the other four were left behind, the Job would
# still (correctly) fail, and scenario B would never truly exercise the
# clean/pass path.
kubectl delete clusterpolicies.kyverno.io "${CR_NAME}" --ignore-not-found >/dev/null
kubectl delete clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUPPOLICY_NAME}" --ignore-not-found >/dev/null
kubectl delete policies.kyverno.io "${POLICY_NAME}" --namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null
kubectl delete cleanuppolicies.kyverno.io "${CLEANUPPOLICY_NAME}" --namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null
kubectl delete policyexceptions.kyverno.io "${POLICYEXCEPTION_NAME}" --namespace "${FIXTURE_NAMESPACE}" --ignore-not-found >/dev/null

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
