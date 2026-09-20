#!/usr/bin/env bash
# Proves the 1.20 legacy-policy migration-grace window (#17490/#17535,
# #17214) is non-destructive across a full upgrade/rollback lifecycle, on a
# live, verified kind cluster with local Kyverno images loaded.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Must match what `make kind-install-kyverno` installs: scenario A/B's
# upgrade steps call back into that target to upgrade this release in place.
RELEASE_NAME="kyverno"
NAMESPACE="kyverno"

HELM="${HELM:-helm}"
LEGACY_CHART_VERSION="${LEGACY_CHART_VERSION:-3.9}"
MAKE="${MAKE:-make}"

CRD_NAMES=(
  clusterpolicies.kyverno.io
  policies.kyverno.io
  cleanuppolicies.kyverno.io
  clustercleanuppolicies.kyverno.io
  policyexceptions.kyverno.io
)

# RUN_ID keeps fixture names unique across runs but doesn't buy concurrency
# safety: the release name, namespace, and cluster-wide legacy counts are
# shared state, so this script still needs exclusive use of the cluster.
RUN_ID="$$-${RANDOM}"
TEST_NAMESPACE="legacy-policy-migration-verify-${RUN_ID}"
CLUSTERPOLICY_NAME="legacy-policy-migration-verify-cp-${RUN_ID}"
SECOND_CLUSTERPOLICY_NAME="legacy-policy-migration-verify-cp2-${RUN_ID}"
# The write-time block is one shared helper (deprecations.ShouldBlock) gated
# per kind by a 5-entry map, called from three separate handlers. All five
# kinds get their own fixture below so each handler and map entry is proven
# wired, not just ClusterPolicy's.
POLICY_NAME="legacy-policy-migration-verify-pol-${RUN_ID}"
SECOND_POLICY_NAME="legacy-policy-migration-verify-pol2-${RUN_ID}"
CLEANUP_POLICY_NAME="legacy-policy-migration-verify-clnp-${RUN_ID}"
SECOND_CLEANUP_POLICY_NAME="legacy-policy-migration-verify-clnp2-${RUN_ID}"
CLUSTERCLEANUP_POLICY_NAME="legacy-policy-migration-verify-ccup-${RUN_ID}"
SECOND_CLUSTERCLEANUP_POLICY_NAME="legacy-policy-migration-verify-ccup2-${RUN_ID}"
POLEX_NAME="legacy-policy-migration-verify-polex-${RUN_ID}"
SECOND_POLEX_NAME="legacy-policy-migration-verify-polex2-${RUN_ID}"
# A CleanupPolicy/ClusterCleanupPolicy is rejected at create time unless its
# service account can delete AND list the matched kind. The chart grants no
# such RBAC by default, so the fixture brings its own, like a real user would.
CLEANUP_RBAC_NAME="legacy-policy-migration-verify-cleanup-rbac-${RUN_ID}"
# Delete probes: throwaway fixtures that exist only to be deleted in A4b (see
# that comment for what it proves). Excluded from every A4/A5 hash/count
# comparison except legacy_clusterpolicy_count() - see the A5 recreation.
DELETE_PROBE_CLUSTERPOLICY_NAME="legacy-policy-migration-verify-delcp-${RUN_ID}"
DELETE_PROBE_CLUSTERCLEANUP_POLICY_NAME="legacy-policy-migration-verify-delccup-${RUN_ID}"
DELETE_PROBE_POLEX_NAME="legacy-policy-migration-verify-delpolex-${RUN_ID}"
VPOL_NAME="legacy-policy-migration-verify-vpol-${RUN_ID}"
VIOLATING_POD_NAME="legacy-policy-migration-verify-violating-${RUN_ID}"
COMPLIANT_POD_NAME="legacy-policy-migration-verify-compliant-${RUN_ID}"

# Distinct deny messages per fixture, so a deny can be attributed to the
# policy that produced it - the ClusterPolicy and namespaced Policy both
# match the same pod in the same namespace, so a shared message wouldn't.
CP_DENY_MSG="label app is required (legacy ClusterPolicy)"
POLICY_DENY_MSG="label app is required (legacy Policy)"
VPOL_DENY_MSG="label app is required (CEL ValidatingPolicy)"

log() { echo "[verify-legacy-policy-migration] $*"; }
fail() { echo "[verify-legacy-policy-migration] FAIL: $*" >&2; exit 1; }

# --- destructive-action guard: must run before anything touches the cluster ---
# A context name starting with "kind-" alone isn't proof - a real cluster
# could be renamed to look disposable. Unless overridden, this also checks
# every node's spec.providerID for kind's own "kind://..." scheme.
CURRENT_CONTEXT="$(kubectl config current-context 2>/dev/null || true)"
if [ -z "${CURRENT_CONTEXT}" ]; then
  fail "could not determine the current kubectl context; refusing to run a script that installs/uninstalls Helm releases and deletes policy CRs against an unknown cluster"
fi

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
[verify-legacy-policy-migration] FAIL: refusing to run against kubectl context '${CURRENT_CONTEXT}'.

This script installs, upgrades, rolls back, and uninstalls a Helm release
named "${RELEASE_NAME}" in the "${NAMESPACE}" namespace, and deletes its own
fixture CRs (ClusterPolicy, Policy, CleanupPolicy, ClusterCleanupPolicy,
PolicyException) as part of its resets. If this context is a real cluster,
that is irreversible data loss.

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
  # Names the context explicitly. Everything below acts on the captured
  # context, so validating whatever happens to be current could check one
  # cluster and then operate on another.
  NODE_PROVIDER_IDS="$(kubectl --context "${CURRENT_CONTEXT}" get nodes -o jsonpath='{.items[*].spec.providerID}' 2>/dev/null || true)"
  if ! all_providerids_are_kind "${NODE_PROVIDER_IDS}"; then
    guard_refuse "Its name starts with \"kind-\", but its nodes did not all verify live as genuine kind nodes (expected every node to report a kind://... spec.providerID; got: '${NODE_PROVIDER_IDS:-<empty or unreachable>}')."
  fi
  log "current kubectl context '${CURRENT_CONTEXT}' verified as a genuine kind cluster (name starts with kind-, and every node's providerID is kind://...), proceeding"
fi

# Pin kubectl and Helm to the guard-validated context, so a mid-run
# switch can't redirect these calls. HELM_KUBECONTEXT is read by helm
# itself, so it also reaches the nested helm call in run_local_upgrade.
kubectl() { command kubectl --context "${CURRENT_CONTEXT}" "$@"; }
export HELM_KUBECONTEXT="${CURRENT_CONTEXT}"

# --- exclusivity precondition: fail fast on a non-empty cluster -------------
# The count/spec assertions below assume the cluster starts with none of the
# five legacy kinds. Check that up front instead of failing later with a
# confusing baseline-count or B3 grep mismatch.

# Counts objects of one legacy kind cluster-wide. $2 is a scope flag ("" for
# cluster-scoped, "-A" for namespaced). An absent CRD is the normal
# fresh-cluster case and reads as zero; any other kubectl failure fails
# closed, since this feeds a guard that must not pass on an unread cluster.
count_legacy_crs() {
  local out rc=0 err
  err="$(mktemp)"
  # shellcheck disable=SC2086 # $2 must expand to zero args when empty, not one empty arg.
  out="$(kubectl get "$1" ${2:-} -o name 2>"${err}")" || rc=$?
  if [ "${rc}" -ne 0 ]; then
    local msg
    msg="$(cat "${err}")"; rm -f "${err}"
    case "${msg}" in
      *"the server doesn't have a resource type"*|*"could not find the requested resource"*|*"NotFound"*)
        echo 0; return 0 ;;
      *)
        fail "count_legacy_crs: could not list ${1} to verify the cluster is empty, so the exclusivity guard cannot pass: ${msg}" ;;
    esac
  fi
  rm -f "${err}"
  [ -z "${out}" ] && { echo 0; return 0; }
  printf '%s\n' "${out}" | wc -l | tr -d ' '
}

# A leftover legacy CR of any kind - from a prior run or something else -
# would otherwise fail scenario B later with a confusing error, so refuse
# up front instead.
for preexisting_probe in \
  "clusterpolicies.kyverno.io|ClusterPolicy|" \
  "policies.kyverno.io|Policy|-A" \
  "cleanuppolicies.kyverno.io|CleanupPolicy|-A" \
  "clustercleanuppolicies.kyverno.io|ClusterCleanupPolicy|" \
  "policyexceptions.kyverno.io|PolicyException|-A"; do
  PREEXISTING_RESOURCE="${preexisting_probe%%|*}"
  PREEXISTING_REST="${preexisting_probe#*|}"
  PREEXISTING_KIND="${PREEXISTING_REST%%|*}"
  PREEXISTING_SCOPE="${PREEXISTING_REST#*|}"
  PREEXISTING_COUNT="$(count_legacy_crs "${PREEXISTING_RESOURCE}" "${PREEXISTING_SCOPE}")"
  if [ "${PREEXISTING_COUNT}" != "0" ]; then
    fail "found ${PREEXISTING_COUNT} pre-existing ${PREEXISTING_KIND} object(s) on this cluster; this script requires exclusive use of the cluster and cannot run correctly alongside them - clear them or use a fresh cluster"
  fi
done

WORK_DIR="$(mktemp -d)"

# On success, cleanup tears everything down. On failure it leaves the
# release, namespace, and fixtures in place so the workflow's Debug-failure
# step can inspect them right after this script exits.
cleanup() {
  local exit_code="${1:-0}"
  if [ "${exit_code}" -eq 0 ]; then
    # A slow uninstall shouldn't fail an otherwise-green run, so this one
    # call site tolerates a non-zero uninstall.
    helm_uninstall_if_present || log "cleanup: helm uninstall of a successful run's release did not complete cleanly (ignored)"
    kubectl delete namespace "${TEST_NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
    # DELETE_PROBE_CLUSTERPOLICY_NAME is included because A5 recreates it;
    # the other two delete probes are already gone, and --ignore-not-found
    # tolerates that.
    kubectl delete clusterpolicies.kyverno.io "${CLUSTERPOLICY_NAME}" "${SECOND_CLUSTERPOLICY_NAME}" "${DELETE_PROBE_CLUSTERPOLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUP_POLICY_NAME}" "${SECOND_CLUSTERCLEANUP_POLICY_NAME}" "${DELETE_PROBE_CLUSTERCLEANUP_POLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete validatingpolicies.policies.kyverno.io "${VPOL_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    # Namespaced fixtures are also deleted by name, in case the namespace
    # delete above timed out or was already gone.
    kubectl delete policies.kyverno.io -n "${TEST_NAMESPACE}" "${POLICY_NAME}" "${SECOND_POLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete cleanuppolicies.kyverno.io -n "${TEST_NAMESPACE}" "${CLEANUP_POLICY_NAME}" "${SECOND_CLEANUP_POLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete policyexceptions.kyverno.io -n "${TEST_NAMESPACE}" "${POLEX_NAME}" "${SECOND_POLEX_NAME}" "${DELETE_PROBE_POLEX_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete clusterrole "${CLEANUP_RBAC_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    rm -rf "${WORK_DIR}"
  else
    log "exiting non-zero (${exit_code}): leaving the \"${RELEASE_NAME}\" release, the \"${NAMESPACE}\"/\"${TEST_NAMESPACE}\" namespaces, this run's fixtures, and \"${WORK_DIR}\" (logs and CRD/webhook snapshots) in place so the workflow's Debug-failure logs step (and manual kubectl inspection) can see the failure state"
  fi
}
trap 'cleanup $?' EXIT

# Does not swallow a genuine uninstall failure: called under `set -e` from
# the scenario A/B reset, where swallowing it would surface later as a
# confusing "name still in use" error from scenario B's own install.
helm_uninstall_if_present() {
  if "${HELM}" status "${RELEASE_NAME}" -n "${NAMESPACE}" >/dev/null 2>&1; then
    "${HELM}" uninstall "${RELEASE_NAME}" -n "${NAMESPACE}" --wait --timeout 2m
  fi
}

# --- fixture manifests -------------------------------------------------------

apply_legacy_fixture() {
  kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: ${CLUSTERPOLICY_NAME}
spec:
  validationFailureAction: Enforce
  rules:
  - name: check-label
    match:
      resources:
        kinds:
        - Pod
        namespaces:
        - ${TEST_NAMESPACE}
    validate:
      message: "${CP_DENY_MSG}"
      pattern:
        metadata:
          labels:
            app: "?*"
EOF
}

# Applies the throwaway delete-probe fixture (see DELETE_PROBE_CLUSTERPOLICY_NAME).
# Called in A1, then again in A5 after the rollback to recreate it.
apply_delete_probe_clusterpolicy() {
  kubectl apply -f - <<EOF >/dev/null
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: ${DELETE_PROBE_CLUSTERPOLICY_NAME}
spec:
  rules: []
EOF
}

# Namespaced twin of apply_legacy_fixture, as a kyverno.io/v1 Policy. Emitted
# rather than applied so callers can apply it directly or stash it to a file
# for the retry helpers.
policy_manifest() {
  # No match.resources.namespaces: a namespaced Policy is already scoped to
  # its own namespace.
  cat <<EOF
apiVersion: kyverno.io/v1
kind: Policy
metadata:
  name: $1
  namespace: ${TEST_NAMESPACE}
spec:
  validationFailureAction: Enforce
  rules:
  - name: check-label
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "${POLICY_DENY_MSG}"
      pattern:
        metadata:
          labels:
            app: "?*"
EOF
}


# apply_cleanup_rbac grants the cleanup controller's service account delete
# and list on Pods, via the chart's own aggregation label. Both verbs are
# required: canI() runs a SubjectAccessReview for each, and either one
# failing rejects the CleanupPolicy at create time on 1.19.
apply_cleanup_rbac() {
  kubectl apply -f - <<EOF >/dev/null
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ${CLEANUP_RBAC_NAME}
  labels:
    rbac.kyverno.io/aggregate-to-cleanup-controller: "true"
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["delete", "list"]
EOF
}

# Emits (rather than applies) the manifest. apiVersion is resolved at
# runtime from the CRD's storage version, since LEGACY_CHART_VERSION floats
# and a hardcoded version would be brittle.
cleanup_policy_manifest() {
  cat <<EOF
apiVersion: kyverno.io/${CLEANUP_POLICY_APIVERSION}
kind: CleanupPolicy
metadata:
  name: $1
  namespace: ${TEST_NAMESPACE}
spec:
  schedule: "0 0 1 1 *"
  match:
    any:
    - resources:
        kinds:
        - Pod
        names:
        - legacy-policy-migration-verify-never-matches
EOF
}

polex_manifest() {
  cat <<EOF
apiVersion: kyverno.io/${POLEX_APIVERSION}
kind: PolicyException
metadata:
  name: $1
  namespace: ${TEST_NAMESPACE}
spec:
  exceptions:
  - policyName: legacy-policy-migration-verify-does-not-exist
    ruleNames:
    - "*"
  match:
    any:
    - resources:
        kinds:
        - Pod
EOF
}

# Cluster-scoped twin of cleanup_policy_manifest: same shape, minus a
# namespace field.
clustercleanuppolicy_manifest() {
  cat <<EOF
apiVersion: kyverno.io/${CLUSTERCLEANUP_POLICY_APIVERSION}
kind: ClusterCleanupPolicy
metadata:
  name: $1
spec:
  schedule: "0 0 1 1 *"
  match:
    any:
    - resources:
        kinds:
        - Pod
        names:
        - legacy-policy-migration-verify-never-matches
EOF
}




# CEL twin of apply_legacy_fixture: a ValidatingPolicy enforcing the same
# require-label rule on Pods in TEST_NAMESPACE.
apply_vpol_fixture() {
  kubectl apply -f - <<EOF >/dev/null
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: ${VPOL_NAME}
spec:
  matchConstraints:
    resourceRules:
    - apiGroups:   [""]
      apiVersions: [v1]
      operations:  [CREATE, UPDATE]
      resources:   [pods]
  validations:
    - expression: >-
        object.metadata.namespace != "${TEST_NAMESPACE}" ||
        (has(object.metadata.labels) && "app" in object.metadata.labels)
      message: "${VPOL_DENY_MSG}"
EOF
}

violating_pod_manifest() {
  cat <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${VIOLATING_POD_NAME}
  namespace: ${TEST_NAMESPACE}
spec:
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.9
EOF
}

compliant_pod_manifest() {
  cat <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${COMPLIANT_POD_NAME}
  namespace: ${TEST_NAMESPACE}
  labels:
    app: ok
spec:
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.9
EOF
}

# --- retry helpers ------------------------------------------------------------
# Right after an install/upgrade/rollback there's a short window before the
# webhook is serving, where an apply can spuriously succeed or fail. These
# helpers retry for a couple of minutes instead of asserting on the first try.

# Asserts applying manifest_file is denied for expected_substr specifically,
# not some unrelated transient error (e.g. a stale caBundle or a webhook not
# registered yet) - both of those are retried instead of failed immediately.
wait_for_deny() {
  local desc="$1" manifest_file="$2" expected_substr="$3"
  local waited=0 timeout=120 interval=3
  while true; do
    if ! kubectl apply -f "${manifest_file}" >"${WORK_DIR}/last-apply.log" 2>&1; then
      if grep -qF -- "${expected_substr}" "${WORK_DIR}/last-apply.log"; then
        return 0
      fi
      # Denied, but not yet for the expected reason - retry instead of
      # failing immediately.
      :
    else
      # Unexpectedly succeeded - force-delete and retry, in case the webhook
      # simply wasn't ready yet.
      kubectl delete -f "${manifest_file}" --ignore-not-found --grace-period=0 --force >/dev/null 2>&1 || true
    fi
    waited=$((waited + interval))
    if [ "${waited}" -ge "${timeout}" ]; then
      cat "${WORK_DIR}/last-apply.log" >&2
      fail "${desc}: expected apply to be denied for '${expected_substr}' within ${timeout}s (see the last captured output above)"
    fi
    sleep "${interval}"
  done
}

wait_for_allow() {
  local desc="$1" manifest_file="$2"
  local waited=0 timeout=120 interval=3
  while true; do
    if kubectl apply -f "${manifest_file}" >"${WORK_DIR}/last-apply.log" 2>&1; then
      return 0
    fi
    waited=$((waited + interval))
    if [ "${waited}" -ge "${timeout}" ]; then
      cat "${WORK_DIR}/last-apply.log" >&2
      fail "${desc}: expected apply to succeed, but it kept failing for ${timeout}s"
    fi
    sleep "${interval}"
  done
}

# Asserts a `kubectl patch --type merge` is denied for expected_substr. On
# an unexpected success, calls recover_fn to restore the pre-patch spec so a
# spurious mutation can't corrupt later hash checks (optional 7th arg: namespace).
wait_for_patch_denied() {
  local desc="$1" resource="$2" name="$3" patch_json="$4" expected_substr="$5" recover_fn="$6" namespace="${7:-}"
  local ns_flag=""
  if [ -n "${namespace}" ]; then
    ns_flag="-n ${namespace}"
  fi
  local waited=0 timeout=120 interval=3
  while true; do
    # shellcheck disable=SC2086 # ns_flag must expand to zero args when empty, not one empty arg.
    if ! kubectl patch ${ns_flag} "${resource}" "${name}" --type merge -p "${patch_json}" >"${WORK_DIR}/last-apply.log" 2>&1; then
      if grep -qF -- "${expected_substr}" "${WORK_DIR}/last-apply.log"; then
        return 0
      fi
      # Denied, but not (yet) for the expected reason - retry rather than
      # failing immediately (see wait_for_deny).
      :
    else
      "${recover_fn}"
    fi
    waited=$((waited + interval))
    if [ "${waited}" -ge "${timeout}" ]; then
      cat "${WORK_DIR}/last-apply.log" >&2
      fail "${desc}: expected patch to be denied for '${expected_substr}' within ${timeout}s (see the last captured output above)"
    fi
    sleep "${interval}"
  done
}

# The optional 5th argument is a namespace; see wait_for_patch_denied.
wait_for_annotate_allowed() {
  local desc="$1" resource="$2" name="$3" annotation="$4" namespace="${5:-}"
  local ns_flag=""
  if [ -n "${namespace}" ]; then
    ns_flag="-n ${namespace}"
  fi
  local waited=0 timeout=120 interval=3
  while true; do
    # shellcheck disable=SC2086 # ns_flag must expand to zero args when empty, not one empty arg.
    if kubectl annotate ${ns_flag} "${resource}" "${name}" "${annotation}" --overwrite >"${WORK_DIR}/last-apply.log" 2>&1; then
      return 0
    fi
    waited=$((waited + interval))
    if [ "${waited}" -ge "${timeout}" ]; then
      cat "${WORK_DIR}/last-apply.log" >&2
      fail "${desc}: expected annotate to succeed, but it kept failing for ${timeout}s"
    fi
    sleep "${interval}"
  done
}

# --- CRD/webhook snapshot helpers --------------------------------------------

# Hashes .spec with "description" fields stripped. Doc-string edits between
# the published 1.19 chart and the local chart are expected noise, not a
# real schema/storage/served-version regression, so this hash must ignore them.
crd_spec_hash() {
  kubectl get crd "$1" -o json \
    | jq -Sc 'walk(if type == "object" then del(.description) else . end) | .spec' \
    | shasum -a 256 | awk '{print $1}'
}

crd_versions_summary() {
  kubectl get crd "$1" -o json | jq -Sc '[.spec.versions[] | {name, served, storage}]'
}

crd_stored_versions() {
  kubectl get crd "$1" -o json | jq -Sc '.status.storedVersions // []'
}

# Writes each legacy CRD's spec hash, versions summary, and storedVersions
# to ${WORK_DIR}/<label>-<crd>.snapshot.
snapshot_crds() {
  local label="$1"
  local crd
  for crd in "${CRD_NAMES[@]}"; do
    local spec_hash versions stored
    spec_hash="$(crd_spec_hash "${crd}")"
    versions="$(crd_versions_summary "${crd}")"
    stored="$(crd_stored_versions "${crd}")"
    {
      echo "specHash=${spec_hash}"
      echo "versions=${versions}"
      echo "storedVersions=${stored}"
    } > "${WORK_DIR}/${label}-${crd}.snapshot"
  done
}

# Compares two previously taken snapshots byte-for-byte, per legacy CRD.
assert_crds_unchanged() {
  local baseline_label="$1" current_label="$2" context="$3"
  local crd
  for crd in "${CRD_NAMES[@]}"; do
    if ! diff -u "${WORK_DIR}/${baseline_label}-${crd}.snapshot" "${WORK_DIR}/${current_label}-${crd}.snapshot" >"${WORK_DIR}/crd-diff.log" 2>&1; then
      cat "${WORK_DIR}/crd-diff.log" >&2
      fail "${context}: CRD ${crd} changed (spec.versions/spec hash/storedVersions) between '${baseline_label}' and '${current_label}'"
    fi
  done
}

# Merges validating and mutating webhook configs into one array, each item
# tagged with __whType. No fallback on a per-type read error: silently
# treating one type as empty would produce a plausible-but-wrong snapshot.
webhook_configs_json() {
  local validating mutating
  validating="$(kubectl get validatingwebhookconfigurations -o json 2>/dev/null \
    | jq -c '[.items[] | . + {__whType: "validating"}]')"
  mutating="$(kubectl get mutatingwebhookconfigurations -o json 2>/dev/null \
    | jq -c '[.items[] | . + {__whType: "mutating"}]')"
  jq -nc --argjson v "${validating}" --argjson m "${mutating}" '$v + $m'
}

# Scans every validating AND mutating webhook config, not just validating:
# buildPolicyMutatingWebhookConfiguration mirrors the same policyRule, and
# losing only the mutating side must not be masked by validating coverage.
legacy_webhook_resources() {
  webhook_configs_json \
    | jq -Sc '
        def legacyKinds: ["clusterpolicies","policies","cleanuppolicies","clustercleanuppolicies","policyexceptions"];
        [ .[].webhooks[]?.rules[]?
          | select((.apiGroups // []) | index("kyverno.io"))
          | (.resources // [])[]?
          | sub("/\\*$"; "")
          | select(. as $r | legacyKinds | index($r) != null)
        ] | unique'
}

# Asserts every legacy kind is covered by some webhook config's rules.
# Retries: configs are reconciled asynchronously after pods go ready, so a
# call right after wait_kyverno_ready can race a cold start.
webhook_rules_cover_legacy_kinds() {
  local context="$1"
  local waited=0 timeout=60 interval=3
  local resources kind missing
  while true; do
    resources="$(legacy_webhook_resources || true)"
    missing=""
    if [ -n "${resources}" ] && [ "${resources}" != "null" ]; then
      for kind in clusterpolicies policies cleanuppolicies clustercleanuppolicies policyexceptions; do
        # Match the bare resource name or a subresource-wildcarded entry,
        # e.g. "cleanuppolicies/*" (the cleanup controller's webhook uses this).
        echo "${resources}" | grep -Eq "\"${kind}(/[^\"]*)?\"" || missing="${missing} ${kind}"
      done
    else
      missing=" <no rules read>"
    fi
    [ -z "${missing}" ] && return 0
    waited=$((waited + interval))
    if [ "${waited}" -ge "${timeout}" ]; then
      echo "resources seen: ${resources}" >&2
      fail "${context}: read no validating or mutating webhookconfiguration rule (apiGroups: kyverno.io) covering:${missing} within ${timeout}s - either none exist or the API stayed unreachable"
    fi
    sleep "${interval}"
  done
}

# Normalized snapshot of every legacy-kind rule, across both validating and
# mutating configs, keyed by type/config/webhook so losing one type isn't
# masked by the other. "/*" normalizes to the bare plural (kept as sub=below).
legacy_webhook_rules_snapshot() {
  webhook_configs_json \
    | jq -Sc '
      def legacyKinds: ["clusterpolicies","policies","cleanuppolicies","clustercleanuppolicies","policyexceptions"];
      [ .[] as $cfg
        | ($cfg.webhooks // [])[] as $wh
        | ($wh.rules // [])[] as $rule
        | select($rule.apiGroups // [] | (index("kyverno.io") or index("*")))
        | (($rule.resources // []) | unique
             | map(select(sub("/\\*$"; "") as $r
                          | ($r == "*") or (legacyKinds | index($r) != null)))) as $legacyResources
        | select(($legacyResources | length) > 0)
        | {
            type: $cfg.__whType,
            config: $cfg.metadata.name,
            webhook: $wh.name,
            apiGroups: ($rule.apiGroups // [] | sort),
            apiVersions: ($rule.apiVersions // [] | sort),
            resources: ($legacyResources | sort),
            operations: ($rule.operations // [] | sort),
            scope: ($rule.scope // "*")
          }
      ] | sort
    '
}

# Writes the webhook-rule snapshot to ${WORK_DIR}/<label>-webhook-rules.snapshot.
# Retries like webhook_rules_cover_legacy_kinds: a call right after
# wait_kyverno_ready can race the async rule reconcile.
snapshot_legacy_webhook_rules() {
  local label="$1"
  local waited=0 timeout=60 interval=3
  local snapshot
  while true; do
    snapshot="$(legacy_webhook_rules_snapshot || true)"
    if [ -n "${snapshot}" ] && [ "${snapshot}" != "[]" ] && [ "${snapshot}" != "null" ]; then
      echo "${snapshot}" > "${WORK_DIR}/${label}-webhook-rules.snapshot"
      return 0
    fi
    waited=$((waited + interval))
    if [ "${waited}" -ge "${timeout}" ]; then
      fail "snapshot_legacy_webhook_rules(${label}): read no legacy-kind webhook rule (validating or mutating) within ${timeout}s - either none exist or the API stayed unreachable"
    fi
    sleep "${interval}"
  done
}

# Compares a fresh webhook-rule snapshot against a saved baseline, retrying
# rather than sampling once: rules reconcile asynchronously after an
# upgrade/rollback, so a transient mismatch isn't itself a failure.
assert_legacy_webhook_rules_unchanged() {
  local baseline_label="$1" current_label="$2" context="$3"
  local baseline_file="${WORK_DIR}/${baseline_label}-webhook-rules.snapshot"
  local current_file="${WORK_DIR}/${current_label}-webhook-rules.snapshot"
  local waited=0 timeout=60 interval=3
  local current
  while true; do
    current="$(legacy_webhook_rules_snapshot || true)"
    echo "${current}" > "${current_file}"
    if diff -u "${baseline_file}" "${current_file}" >"${WORK_DIR}/webhook-rules-diff.log" 2>&1; then
      return 0
    fi
    waited=$((waited + interval))
    if [ "${waited}" -ge "${timeout}" ]; then
      cat "${WORK_DIR}/webhook-rules-diff.log" >&2
      fail "${context}: legacy-kind webhook rules (validating and mutating) still differ from '${baseline_label}' after ${timeout}s (apiGroups/apiVersions/resources/operations/scope, or which type/config/webhook owns them) - see the diff above; an empty current side means the API stayed unreachable"
    fi
    sleep "${interval}"
  done
}

# Reduces a legacy_webhook_rules_snapshot array to, per legacy kind, the
# sorted set of "type|apiVersion|operation|scope|sub=" capabilities it is
# covered for. A4 tolerates gaining one (e.g. a storage-version fix); only losing one is a regression.
legacy_webhook_capabilities() {
  jq -Sc '
    reduce (.[] | . as $rule | $rule.resources[] as $raw
      | ($raw | sub("/\\*$"; "")) as $kind
      | ($raw | endswith("/*")) as $wildcard
      | $rule.apiVersions[] as $av | $rule.operations[] as $op
      | {kind: $kind,
         triple: ($rule.type + "|" + $av + "|" + $op + "|" + $rule.scope + "|sub=" + ($wildcard | tostring))}) as $e
      ({}; .[$e.kind] += [$e.triple])
    | map_values(unique)
  '
}

# Like assert_legacy_webhook_rules_unchanged, but tolerates additions: fails
# only if a baseline (kind, type, apiVersion, operation, scope, sub)
# capability is lost - type is exact, so a missing mutating rule can't hide behind a still-covering validating one.
assert_legacy_webhook_rules_not_narrowed() {
  local baseline_label="$1" current_label="$2" context="$3"
  local baseline_file="${WORK_DIR}/${baseline_label}-webhook-rules.snapshot"
  local current_file="${WORK_DIR}/${current_label}-webhook-rules.snapshot"
  local baseline_caps
  baseline_caps="$(legacy_webhook_capabilities < "${baseline_file}")"
  local waited=0 timeout=60 interval=3
  local current current_caps lost
  while true; do
    current="$(legacy_webhook_rules_snapshot || true)"
    lost=""
    if [ -n "${current}" ] && jq -e . >/dev/null 2>&1 <<< "${current}"; then
      echo "${current}" > "${current_file}"
      current_caps="$(legacy_webhook_capabilities <<< "${current}")"
      lost="$(jq -n --argjson base "${baseline_caps}" --argjson cur "${current_caps}" '
      def covers($c; $b):
        ($c[0] == $b[0])
        and ($c[1] == $b[1] or $c[1] == "*")
        and ($c[2] == $b[2] or $c[2] == "*")
        and ($c[3] == $b[3] or $c[3] == "*")
        and ($c[4] == $b[4] or $c[4] == "sub=true");
      [ ($base | keys[]) as $k
        | (($cur[$k] // []) | map(split("|"))) as $curTriples
        | ($base[$k] | map(select(. as $t | ($t | split("|")) as $bt
            | ($curTriples | any(covers(.; $bt))) | not))) as $l
        | select(($l | length) > 0)
        | {kind: $k, lost: $l}
      ]')"
    fi
    if [ "${lost}" = "[]" ]; then
      return 0
    fi
    waited=$((waited + interval))
    if [ "${waited}" -ge "${timeout}" ]; then
      echo "lost coverage: ${lost}" >&2
      fail "${context}: legacy-kind webhook coverage still differs from '${baseline_label}' after ${timeout}s (type|apiVersion|operation|scope|sub triple(s) missing) - see above; an empty current side means the API stayed unreachable"
    fi
    sleep "${interval}"
  done
}

# Hashes only .spec, not the whole object, so unrelated metadata/annotation
# churn on the CR doesn't move the hash.
clusterpolicy_spec_hash() {
  kubectl get clusterpolicy "$1" -o json | jq -Sc '.spec' | shasum -a 256 | awk '{print $1}'
}

# Returns the one version a CRD marks storage:true. Used to build the
# CleanupPolicy/PolicyException fixtures against the version the installed
# chart actually serves.
crd_storage_version() {
  kubectl get crd "$1" -o jsonpath='{.spec.versions[?(@.storage==true)].name}'
}

# TEST_NAMESPACE-scoped twin of clusterpolicy_spec_hash, for the Policy,
# CleanupPolicy, and PolicyException fixtures.
namespaced_spec_hash() {
  kubectl get "$1" "$2" -n "${TEST_NAMESPACE}" -o json | jq -Sc '.spec' | shasum -a 256 | awk '{print $1}'
}

# Generic (kind-parameterized) twin of clusterpolicy_spec_hash, for the
# ClusterCleanupPolicy fixture.
clusterscoped_spec_hash() {
  kubectl get "$1" "$2" -o json | jq -Sc '.spec' | shasum -a 256 | awk '{print $1}'
}

# Counts every ClusterPolicy on the cluster, so a rollback that duplicates
# or resurrects a stray legacy CR is caught even if the fixture's own
# object still looks untouched.
legacy_clusterpolicy_count() {
  kubectl get clusterpolicies.kyverno.io -o name | wc -l | tr -d ' '
}

# Asserts a delete probe is genuinely absent, not just trusted absent:
# a failed GET (not NotFound) must not read as "absent". Extra args after
# desc/name are passed straight to `kubectl get`.
assert_delete_probe_absent() {
  local desc="$1" name="$2"
  shift 2
  local err
  err="$(mktemp)"
  if kubectl get "$@" "${name}" >/dev/null 2>"${err}"; then
    rm -f "${err}"
    fail "${desc}: delete probe '${name}' still exists post-rollback, even though A4b deleted it under the active 1.20 write-block - the rollback resurrected or never actually removed it"
  fi
  case "$(cat "${err}")" in
    *NotFound*|*"could not find the requested resource"*|*"the server doesn't have a resource type"*) ;;
    *) rm -f "${err}"; fail "${desc}: could not confirm delete probe '${name}' is absent - the read itself failed, so a resurrected object could be masked" ;;
  esac
  rm -f "${err}"
}

# --- upgrade helper -----------------------------------------------------------
# Reuses `make kind-install-kyverno` to upgrade the release in place to the
# local chart. EXPLICIT_INSTALL_SETTINGS carries the grace opt-out (or is
# empty, for the shipped default), passed as a prefix scoping it to this call.
run_local_upgrade() {
  local explicit_settings="$1" out_file="$2"
  (
    cd "${ROOT_DIR}" &&
      HELM="${HELM}" EXPLICIT_INSTALL_SETTINGS="${explicit_settings}" \
        "${MAKE}" kind-install-kyverno
  ) >"${out_file}" 2>&1
}

# Uses $SECONDS as a wall-clock deadline, not a counter: `kubectl wait` can
# fail instantly (e.g. "no matching resources found"), and a counter would
# let the loop burn the whole budget in under a second.
wait_kyverno_ready() {
  local timeout=600 attempt_timeout=20 poll_interval=5
  local deadline=$((SECONDS + timeout))
  while true; do
    if kubectl wait --namespace "${NAMESPACE}" --for=condition=ready pod --selector '!job-name' --timeout="${attempt_timeout}s" >"${WORK_DIR}/last-wait.log" 2>&1; then
      return 0
    fi
    if [ "${SECONDS}" -ge "${deadline}" ]; then
      cat "${WORK_DIR}/last-wait.log" >&2
      kubectl get pods -n "${NAMESPACE}" -o wide >&2 || true
      fail "kyverno pods not all ready within ${timeout}s (see the last captured output and pod list above)"
    fi
    sleep "${poll_interval}"
  done
}

# Every Kyverno controller Deployment's image, keyed by its
# app.kubernetes.io/component label (admission/cleanup/reports/background),
# not just the admission controller: a partial `helm upgrade` could leave
# one controller's manifest unapplied while changing the others.
controller_deployment_images() {
  kubectl get deployment -n "${NAMESPACE}" -l "app.kubernetes.io/instance=${RELEASE_NAME}" -o json 2>/dev/null \
    | jq -Sc '[.items[] | select(.metadata.labels["app.kubernetes.io/component"] != null)
        | {component: .metadata.labels["app.kubernetes.io/component"], image: .spec.template.spec.containers[0].image}]
        | sort_by(.component)' \
    2>/dev/null || true
}

release_revision() {
  "${HELM}" history "${RELEASE_NAME}" -n "${NAMESPACE}" -o json 2>/dev/null | jq -r '.[-1].revision' 2>/dev/null || true
}

release_status() {
  "${HELM}" status "${RELEASE_NAME}" -n "${NAMESPACE}" -o json 2>/dev/null | jq -r '.info.status' 2>/dev/null || true
}

# ==============================================================================
# Scenario A: grace opt-out upgrade, then rollback
# ==============================================================================

log "=== scenario A: grace opt-out upgrade, then rollback ==="

log "A1: installing published 1.19 chart (version ${LEGACY_CHART_VERSION}), applying the legacy fixture"
kubectl create namespace "${TEST_NAMESPACE}" >/dev/null
"${HELM}" install "${RELEASE_NAME}" --repo https://kyverno.github.io/kyverno kyverno \
  --version "${LEGACY_CHART_VERSION}" -n "${NAMESPACE}" --create-namespace --wait --timeout 5m
wait_kyverno_ready
apply_legacy_fixture

violating_pod_manifest > "${WORK_DIR}/violating-pod.yaml"
compliant_pod_manifest > "${WORK_DIR}/compliant-pod.yaml"

log "A1: applying the Policy, CleanupPolicy, ClusterCleanupPolicy, and PolicyException fixtures (the other four legacy kinds)"
CLEANUP_POLICY_APIVERSION="$(crd_storage_version cleanuppolicies.kyverno.io)"
CLUSTERCLEANUP_POLICY_APIVERSION="$(crd_storage_version clustercleanuppolicies.kyverno.io)"
POLEX_APIVERSION="$(crd_storage_version policyexceptions.kyverno.io)"
[ -n "${CLEANUP_POLICY_APIVERSION}" ] || fail "A1: could not resolve the storage version of cleanuppolicies.kyverno.io"
[ -n "${CLUSTERCLEANUP_POLICY_APIVERSION}" ] || fail "A1: could not resolve the storage version of clustercleanuppolicies.kyverno.io"
[ -n "${POLEX_APIVERSION}" ] || fail "A1: could not resolve the storage version of policyexceptions.kyverno.io"
log "A1: resolved legacy fixture versions - CleanupPolicy=kyverno.io/${CLEANUP_POLICY_APIVERSION}, ClusterCleanupPolicy=kyverno.io/${CLUSTERCLEANUP_POLICY_APIVERSION}, PolicyException=kyverno.io/${POLEX_APIVERSION}"

apply_cleanup_rbac
policy_manifest "${POLICY_NAME}" > "${WORK_DIR}/policy.yaml"
policy_manifest "${SECOND_POLICY_NAME}" > "${WORK_DIR}/new-policy.yaml"
cleanup_policy_manifest "${CLEANUP_POLICY_NAME}" > "${WORK_DIR}/cleanup-policy.yaml"
polex_manifest "${POLEX_NAME}" > "${WORK_DIR}/polex.yaml"
cleanup_policy_manifest "${SECOND_CLEANUP_POLICY_NAME}" > "${WORK_DIR}/new-cleanup-policy.yaml"
polex_manifest "${SECOND_POLEX_NAME}" > "${WORK_DIR}/new-polex.yaml"
clustercleanuppolicy_manifest "${CLUSTERCLEANUP_POLICY_NAME}" > "${WORK_DIR}/clustercleanup-policy.yaml"
clustercleanuppolicy_manifest "${SECOND_CLUSTERCLEANUP_POLICY_NAME}" > "${WORK_DIR}/new-clustercleanup-policy.yaml"
# Policy goes through the same handler as ClusterPolicy and needs no extra
# RBAC, so a plain apply (matching apply_legacy_fixture above) is enough.
kubectl apply -f "${WORK_DIR}/policy.yaml" >/dev/null
# wait_for_allow, not a bare apply: RBAC aggregation for the ClusterRole
# just applied is reconciled asynchronously by kube-controller-manager.
wait_for_allow "A1 cleanup policy fixture create" "${WORK_DIR}/cleanup-policy.yaml"
wait_for_allow "A1 cluster cleanup policy fixture create" "${WORK_DIR}/clustercleanup-policy.yaml"
wait_for_allow "A1 policy exception fixture create" "${WORK_DIR}/polex.yaml"

log "A1: applying dedicated delete-probe fixtures (ClusterPolicy, ClusterCleanupPolicy, PolicyException) - these exist only to be deleted in A4b while the 1.20 write-block is active, and are excluded from every hash/count assertion above and below"
clustercleanuppolicy_manifest "${DELETE_PROBE_CLUSTERCLEANUP_POLICY_NAME}" > "${WORK_DIR}/delete-probe-clustercleanup-policy.yaml"
polex_manifest "${DELETE_PROBE_POLEX_NAME}" > "${WORK_DIR}/delete-probe-polex.yaml"
# Plain apply, no RBAC dependency: a create failure aborts under `set -e`,
# which is what proves the probe exists before A4b tries to delete it.
apply_delete_probe_clusterpolicy
# wait_for_allow, not a bare apply: same RBAC-aggregation race as the
# ClusterCleanupPolicy/PolicyException fixtures above.
wait_for_allow "A1 delete-probe cluster cleanup policy fixture create" "${WORK_DIR}/delete-probe-clustercleanup-policy.yaml"
wait_for_allow "A1 delete-probe policy exception fixture create" "${WORK_DIR}/delete-probe-polex.yaml"

log "A1: verifying the legacy policies enforce on 1.19 (violating pod denied by both fixtures, compliant pod admitted)"
wait_for_deny "A1 pre-upgrade ClusterPolicy enforcement" "${WORK_DIR}/violating-pod.yaml" "${CP_DENY_MSG}"
wait_for_deny "A1 pre-upgrade Policy enforcement" "${WORK_DIR}/violating-pod.yaml" "${POLICY_DENY_MSG}"
wait_for_allow "A1 pre-upgrade compliant pod" "${WORK_DIR}/compliant-pod.yaml"
kubectl delete -f "${WORK_DIR}/compliant-pod.yaml" --ignore-not-found >/dev/null

log "A2: snapshotting the baseline (CRD versions/spec hash/storedVersions, ClusterPolicy spec/count, webhook rules)"
snapshot_crds "baseline"
BASELINE_CLUSTERPOLICY_HASH="$(clusterpolicy_spec_hash "${CLUSTERPOLICY_NAME}")"
BASELINE_CLUSTERPOLICY_COUNT="$(legacy_clusterpolicy_count)"
BASELINE_POLICY_HASH="$(namespaced_spec_hash policies.kyverno.io "${POLICY_NAME}")"
BASELINE_CLEANUP_POLICY_HASH="$(namespaced_spec_hash cleanuppolicy "${CLEANUP_POLICY_NAME}")"
BASELINE_CLUSTERCLEANUP_POLICY_HASH="$(clusterscoped_spec_hash clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUP_POLICY_NAME}")"
BASELINE_POLEX_HASH="$(namespaced_spec_hash policyexceptions.kyverno.io "${POLEX_NAME}")"
webhook_rules_cover_legacy_kinds "A2 baseline"
snapshot_legacy_webhook_rules "baseline"

log "A3: upgrading to the LOCAL chart with upgrade.allowLegacyPolicies=true (default write-block left ON)"
run_local_upgrade "--set upgrade.allowLegacyPolicies=true" "${WORK_DIR}/a3-upgrade.log" \
  || { cat "${WORK_DIR}/a3-upgrade.log" >&2; fail "A3: opt-out upgrade to the local chart was expected to succeed"; }
wait_kyverno_ready

log "A4: post-upgrade assertions"
POST_UPGRADE_CLUSTERPOLICY_HASH="$(clusterpolicy_spec_hash "${CLUSTERPOLICY_NAME}")"
if [ "${POST_UPGRADE_CLUSTERPOLICY_HASH}" != "${BASELINE_CLUSTERPOLICY_HASH}" ]; then
  fail "A4: the pre-existing ClusterPolicy's spec changed across the opt-out upgrade"
fi
POST_UPGRADE_CLUSTERPOLICY_COUNT="$(legacy_clusterpolicy_count)"
[ "${POST_UPGRADE_CLUSTERPOLICY_COUNT}" = "${BASELINE_CLUSTERPOLICY_COUNT}" ] \
  || fail "A4: the legacy ClusterPolicy count changed across the opt-out upgrade (baseline=${BASELINE_CLUSTERPOLICY_COUNT}, now=${POST_UPGRADE_CLUSTERPOLICY_COUNT})"
wait_for_deny "A4 ClusterPolicy enforcement still active post-upgrade" "${WORK_DIR}/violating-pod.yaml" "${CP_DENY_MSG}"
wait_for_deny "A4 Policy enforcement still active post-upgrade" "${WORK_DIR}/violating-pod.yaml" "${POLICY_DENY_MSG}"

cat <<EOF > "${WORK_DIR}/new-legacy-policy.yaml"
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: ${SECOND_CLUSTERPOLICY_NAME}
spec:
  rules: []
EOF
# Reuses wait_for_deny for the create-block check - it only cares that
# "kubectl apply -f" is expected to be denied, not the manifest's kind.
wait_for_deny "A4 create blocked" "${WORK_DIR}/new-legacy-policy.yaml" "no longer accepted for create"

# Re-applies the canonical fixture spec if the spec-changing patch below
# unexpectedly succeeds, so a spurious mutation can't corrupt later hash
# comparisons (A5's rollback check in particular).
a4_recover_clusterpolicy_spec() {
  # This revert is itself a spec-changing update, so it can also be denied
  # in the tiny window where the patch succeeded and the webhook then came
  # up. Report that plainly instead of letting a raw kubectl error abort.
  apply_legacy_fixture \
    || fail "A4 spec-update-blocked recovery: could not re-apply the fixture spec (most likely denied by the write-time block under test, in a race between the spurious patch success and the webhook becoming ready - see the kubectl error above); this is a flake window, not a real regression, rerun"
}
SPEC_PATCH='{"spec":{"rules":[{"name":"check-label","match":{"resources":{"kinds":["Pod"],"namespaces":["'"${TEST_NAMESPACE}"'"]}},"validate":{"message":"changed","pattern":{"metadata":{"labels":{"app":"?*"}}}}}]}}'
wait_for_patch_denied "A4 spec-update blocked" "clusterpolicy" "${CLUSTERPOLICY_NAME}" "${SPEC_PATCH}" "no longer accepted" a4_recover_clusterpolicy_spec

wait_for_annotate_allowed "A4 metadata patch allowed" "clusterpolicy" "${CLUSTERPOLICY_NAME}" "legacy-policy-migration-verify/probe=1"
# kubectl get returns the served version, not the storage version - this
# only proves v1 is still served. Storage-version stability is checked
# separately by the CRD snapshot comparison below.
SERVED_APIVERSION="$(kubectl get clusterpolicy "${CLUSTERPOLICY_NAME}" -o jsonpath='{.apiVersion}')"
[ "${SERVED_APIVERSION}" = "kyverno.io/v1" ] \
  || fail "A4: the ClusterPolicy is no longer served as kyverno.io/v1 (got '${SERVED_APIVERSION}'); see the CRD snapshot check below for storage-version stability"

# --- A4, Policy (the namespaced twin, same handler as ClusterPolicy) -------
#
# Same three assertions as ClusterPolicy above, proving the policy handler
# blocks both scopes (the "Policy" map entry, not just "ClusterPolicy").
log "A4: asserting the Policy fixture (namespaced twin of ClusterPolicy) is blocked too"

POST_UPGRADE_POLICY_HASH="$(namespaced_spec_hash policies.kyverno.io "${POLICY_NAME}")"
[ "${POST_UPGRADE_POLICY_HASH}" = "${BASELINE_POLICY_HASH}" ] \
  || fail "A4: the pre-existing Policy's spec changed across the opt-out upgrade"

wait_for_deny "A4 policy create blocked" "${WORK_DIR}/new-policy.yaml" "no longer accepted for create"

a4_recover_policy_spec() {
  kubectl apply -f "${WORK_DIR}/policy.yaml" >/dev/null \
    || fail "A4 policy spec-update-blocked recovery: could not re-apply the fixture spec (most likely denied by the write-time block under test, in a race between the spurious patch success and the webhook becoming ready); this is a flake window, not a real regression, rerun"
}
POLICY_SPEC_PATCH='{"spec":{"rules":[{"name":"check-label","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"changed","pattern":{"metadata":{"labels":{"app":"?*"}}}}}]}}'
wait_for_patch_denied "A4 policy spec-update blocked" "policies.kyverno.io" "${POLICY_NAME}" "${POLICY_SPEC_PATCH}" "no longer accepted" a4_recover_policy_spec "${TEST_NAMESPACE}"

wait_for_annotate_allowed "A4 policy metadata patch allowed" "policies.kyverno.io" "${POLICY_NAME}" "legacy-policy-migration-verify/probe=1" "${TEST_NAMESPACE}"

# --- A4, other two handlers -------------------------------------------------
# Same three assertions, against the other two ShouldBlock call sites. The
# CleanupPolicy/ClusterCleanupPolicy pair matters most: the cleanup
# controller is a separate binary with its own --blockLegacyPolicyAPIs.
log "A4: asserting the CleanupPolicy, ClusterCleanupPolicy, and PolicyException handlers block writes too"

POST_UPGRADE_CLEANUP_POLICY_HASH="$(namespaced_spec_hash cleanuppolicy "${CLEANUP_POLICY_NAME}")"
[ "${POST_UPGRADE_CLEANUP_POLICY_HASH}" = "${BASELINE_CLEANUP_POLICY_HASH}" ] \
  || fail "A4: the pre-existing CleanupPolicy's spec changed across the opt-out upgrade"
POST_UPGRADE_CLUSTERCLEANUP_POLICY_HASH="$(clusterscoped_spec_hash clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUP_POLICY_NAME}")"
[ "${POST_UPGRADE_CLUSTERCLEANUP_POLICY_HASH}" = "${BASELINE_CLUSTERCLEANUP_POLICY_HASH}" ] \
  || fail "A4: the pre-existing ClusterCleanupPolicy's spec changed across the opt-out upgrade"
POST_UPGRADE_POLEX_HASH="$(namespaced_spec_hash policyexceptions.kyverno.io "${POLEX_NAME}")"
[ "${POST_UPGRADE_POLEX_HASH}" = "${BASELINE_POLEX_HASH}" ] \
  || fail "A4: the pre-existing PolicyException's spec changed across the opt-out upgrade"

wait_for_deny "A4 cleanup policy create blocked" "${WORK_DIR}/new-cleanup-policy.yaml" "no longer accepted for create"
wait_for_deny "A4 cluster cleanup policy create blocked" "${WORK_DIR}/new-clustercleanup-policy.yaml" "no longer accepted for create"
wait_for_deny "A4 policy exception create blocked" "${WORK_DIR}/new-polex.yaml" "no longer accepted for create"

# One recover_fn per kind, same reason as a4_recover_clusterpolicy_spec.
a4_recover_cleanup_policy_spec() {
  kubectl apply -f "${WORK_DIR}/cleanup-policy.yaml" >/dev/null \
    || fail "A4 cleanup-policy spec-update-blocked recovery: could not re-apply the fixture spec (most likely denied by the write-time block under test, in a race between the spurious patch success and the webhook becoming ready); this is a flake window, not a real regression, rerun"
}
a4_recover_clustercleanup_policy_spec() {
  kubectl apply -f "${WORK_DIR}/clustercleanup-policy.yaml" >/dev/null \
    || fail "A4 cluster-cleanup-policy spec-update-blocked recovery: could not re-apply the fixture spec (most likely denied by the write-time block under test, in a race between the spurious patch success and the webhook becoming ready); this is a flake window, not a real regression, rerun"
}
a4_recover_polex_spec() {
  kubectl apply -f "${WORK_DIR}/polex.yaml" >/dev/null \
    || fail "A4 policy-exception spec-update-blocked recovery: could not re-apply the fixture spec (most likely denied by the write-time block under test, in a race between the spurious patch success and the webhook becoming ready); this is a flake window, not a real regression, rerun"
}

CLEANUP_SPEC_PATCH='{"spec":{"schedule":"0 0 2 1 *"}}'
wait_for_patch_denied "A4 cleanup policy spec-update blocked" "cleanuppolicy" "${CLEANUP_POLICY_NAME}" "${CLEANUP_SPEC_PATCH}" "no longer accepted" a4_recover_cleanup_policy_spec "${TEST_NAMESPACE}"
CLUSTERCLEANUP_SPEC_PATCH='{"spec":{"schedule":"0 0 2 1 *"}}'
wait_for_patch_denied "A4 cluster cleanup policy spec-update blocked" "clustercleanuppolicies.kyverno.io" "${CLUSTERCLEANUP_POLICY_NAME}" "${CLUSTERCLEANUP_SPEC_PATCH}" "no longer accepted" a4_recover_clustercleanup_policy_spec
POLEX_SPEC_PATCH='{"spec":{"exceptions":[{"policyName":"legacy-policy-migration-verify-changed","ruleNames":["*"]}]}}'
wait_for_patch_denied "A4 policy exception spec-update blocked" "policyexceptions.kyverno.io" "${POLEX_NAME}" "${POLEX_SPEC_PATCH}" "no longer accepted" a4_recover_polex_spec "${TEST_NAMESPACE}"

wait_for_annotate_allowed "A4 cleanup policy metadata patch allowed" "cleanuppolicy" "${CLEANUP_POLICY_NAME}" "legacy-policy-migration-verify/probe=1" "${TEST_NAMESPACE}"
wait_for_annotate_allowed "A4 cluster cleanup policy metadata patch allowed" "clustercleanuppolicies.kyverno.io" "${CLUSTERCLEANUP_POLICY_NAME}" "legacy-policy-migration-verify/probe=1"
wait_for_annotate_allowed "A4 policy exception metadata patch allowed" "policyexceptions.kyverno.io" "${POLEX_NAME}" "legacy-policy-migration-verify/probe=1" "${TEST_NAMESPACE}"

snapshot_crds "post-upgrade"
assert_crds_unchanged "baseline" "post-upgrade" "A4"
# Not strict equality: baseline is the published 1.19 chart and this is
# the local chart, so a legitimate capability addition must not fail this.
assert_legacy_webhook_rules_not_narrowed "baseline" "post-upgrade" "A4"
log "A4: PASS (enforcement intact; create/spec-update blocked and metadata patch allowed on all five legacy kinds across all three block handlers; CRDs unchanged, webhook coverage not narrowed)"

# --- A4b: legacy-policy deletes keep succeeding while the write-block is active
# Proves an operator can still delete a legacy policy on 1.20 to migrate off
# it. Doesn't exercise block.go's Delete/Connect arm: webhooks register only
# CREATE/UPDATE, so DELETE never reaches ShouldBlock (block_test.go covers it).
log "A4b: asserting legacy-policy deletes still succeed against the live 1.20 cluster while the write-block is active"

kubectl delete clusterpolicy "${DELETE_PROBE_CLUSTERPOLICY_NAME}" \
  || fail "A4b: delete of legacy ClusterPolicy '${DELETE_PROBE_CLUSTERPOLICY_NAME}' failed; this is not proof of a block-handler denial (DELETE never reaches pkg/deprecations.ShouldBlock - see the comment above A4b), most likely the probe was missing or already deleted"
kubectl delete clustercleanuppolicies.kyverno.io "${DELETE_PROBE_CLUSTERCLEANUP_POLICY_NAME}" \
  || fail "A4b: delete of legacy ClusterCleanupPolicy '${DELETE_PROBE_CLUSTERCLEANUP_POLICY_NAME}' failed; this is not proof of a block-handler denial (DELETE never reaches pkg/deprecations.ShouldBlock - see the comment above A4b), most likely the probe was missing or already deleted"
kubectl delete policyexceptions.kyverno.io -n "${TEST_NAMESPACE}" "${DELETE_PROBE_POLEX_NAME}" \
  || fail "A4b: delete of legacy PolicyException '${DELETE_PROBE_POLEX_NAME}' failed; this is not proof of a block-handler denial (DELETE never reaches pkg/deprecations.ShouldBlock - see the comment above A4b), most likely the probe was missing or already deleted"

log "A4b: PASS (deletes of legacy ClusterPolicy, ClusterCleanupPolicy, and PolicyException all succeeded against a live 1.20 cluster with the write-block active; DELETE itself does not reach any block handler today - see pkg/deprecations/block_test.go for that coverage)"

log "A5: rolling back to revision 1 (the 1.19 chart)"
"${HELM}" rollback "${RELEASE_NAME}" 1 -n "${NAMESPACE}" --wait --timeout 5m
wait_kyverno_ready

# Confirm all three A4b delete probes are genuinely absent: a blind
# re-apply would be an idempotent no-op if a rollback resurrected one. The
# other two probes are never recreated, so this is their only absence check.
assert_delete_probe_absent "A5" "${DELETE_PROBE_CLUSTERPOLICY_NAME}" clusterpolicy
assert_delete_probe_absent "A5" "${DELETE_PROBE_CLUSTERCLEANUP_POLICY_NAME}" clustercleanuppolicies.kyverno.io
assert_delete_probe_absent "A5" "${DELETE_PROBE_POLEX_NAME}" policyexceptions.kyverno.io -n "${TEST_NAMESPACE}"

# Recreate it now that 1.19 unblocks creates again (couldn't be done any
# earlier). It's counted in the A2 baseline, so without this the
# post-rollback count check below would see baseline-minus-one and fail.
log "A5: recreating the ClusterPolicy delete-probe deleted in A4b, so the count assertion below still matches the untouched baseline"
apply_delete_probe_clusterpolicy

log "A5: post-rollback assertions"
POST_ROLLBACK_CLUSTERPOLICY_HASH="$(clusterpolicy_spec_hash "${CLUSTERPOLICY_NAME}")"
if [ "${POST_ROLLBACK_CLUSTERPOLICY_HASH}" != "${POST_UPGRADE_CLUSTERPOLICY_HASH}" ]; then
  fail "A5: the ClusterPolicy's spec changed across the rollback (it should carry over the A4 metadata annotation untouched, and the spec itself must be identical)"
fi
POST_ROLLBACK_CLUSTERPOLICY_COUNT="$(legacy_clusterpolicy_count)"
[ "${POST_ROLLBACK_CLUSTERPOLICY_COUNT}" = "${BASELINE_CLUSTERPOLICY_COUNT}" ] \
  || fail "A5: the legacy ClusterPolicy count changed across the rollback (baseline=${BASELINE_CLUSTERPOLICY_COUNT}, now=${POST_ROLLBACK_CLUSTERPOLICY_COUNT})"
wait_for_deny "A5 ClusterPolicy enforcement still active post-rollback" "${WORK_DIR}/violating-pod.yaml" "${CP_DENY_MSG}"
wait_for_deny "A5 Policy enforcement still active post-rollback" "${WORK_DIR}/violating-pod.yaml" "${POLICY_DENY_MSG}"
snapshot_crds "post-rollback"
assert_crds_unchanged "baseline" "post-rollback" "A5"
assert_legacy_webhook_rules_unchanged "baseline" "post-rollback" "A5"

# 1.19 has no write-time block, so a second legacy ClusterPolicy must now be
# creatable - proving the block itself rolled back along with the image, not
# just that nothing else regressed.
wait_for_allow "A5 create succeeds on 1.19 (no write-time block)" "${WORK_DIR}/new-legacy-policy.yaml"
kubectl delete clusterpolicy "${SECOND_CLUSTERPOLICY_NAME}" --ignore-not-found >/dev/null

POST_ROLLBACK_POLICY_HASH="$(namespaced_spec_hash policies.kyverno.io "${POLICY_NAME}")"
[ "${POST_ROLLBACK_POLICY_HASH}" = "${POST_UPGRADE_POLICY_HASH}" ] \
  || fail "A5: the Policy's spec changed across the rollback"
POST_ROLLBACK_CLEANUP_POLICY_HASH="$(namespaced_spec_hash cleanuppolicy "${CLEANUP_POLICY_NAME}")"
[ "${POST_ROLLBACK_CLEANUP_POLICY_HASH}" = "${POST_UPGRADE_CLEANUP_POLICY_HASH}" ] \
  || fail "A5: the CleanupPolicy's spec changed across the rollback"
POST_ROLLBACK_CLUSTERCLEANUP_POLICY_HASH="$(clusterscoped_spec_hash clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUP_POLICY_NAME}")"
[ "${POST_ROLLBACK_CLUSTERCLEANUP_POLICY_HASH}" = "${POST_UPGRADE_CLUSTERCLEANUP_POLICY_HASH}" ] \
  || fail "A5: the ClusterCleanupPolicy's spec changed across the rollback"
POST_ROLLBACK_POLEX_HASH="$(namespaced_spec_hash policyexceptions.kyverno.io "${POLEX_NAME}")"
[ "${POST_ROLLBACK_POLEX_HASH}" = "${POST_UPGRADE_POLEX_HASH}" ] \
  || fail "A5: the PolicyException's spec changed across the rollback"

# Only CleanupPolicy gets this check: it proves the cleanup controller's
# own block rolled back with its own image, unlike PolicyException which
# ships in the admission-controller image already proven above.
wait_for_allow "A5 cleanup policy create succeeds on 1.19 (no write-time block)" "${WORK_DIR}/new-cleanup-policy.yaml"
kubectl delete cleanuppolicy -n "${TEST_NAMESPACE}" "${SECOND_CLEANUP_POLICY_NAME}" --ignore-not-found >/dev/null
log "A5: PASS (CR count/spec, CRDs, webhook rules, and enforcement unchanged; write-time block itself rolled back in both controllers)"

log "=== scenario A: PASS ==="

# ==============================================================================
# Reset between scenarios
# ==============================================================================

log "=== resetting for scenario B ==="
# DELETE_PROBE_CLUSTERPOLICY_NAME is included here because A5 recreated it
# (see the comment there); the other two delete probes were already
# permanently deleted in A4b and --ignore-not-found tolerates that.
kubectl delete clusterpolicy "${CLUSTERPOLICY_NAME}" "${DELETE_PROBE_CLUSTERPOLICY_NAME}" --ignore-not-found >/dev/null
kubectl delete clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUP_POLICY_NAME}" "${SECOND_CLUSTERCLEANUP_POLICY_NAME}" "${DELETE_PROBE_CLUSTERCLEANUP_POLICY_NAME}" --ignore-not-found >/dev/null
# TEST_NAMESPACE survives this reset, so namespaced fixtures need explicit
# deletion. Scenario B stays ClusterPolicy-only, so clearing all five kinds
# here keeps B3's "- ClusterPolicy: 1" grep an exact single-kind match.
kubectl delete policies.kyverno.io -n "${TEST_NAMESPACE}" "${POLICY_NAME}" "${SECOND_POLICY_NAME}" --ignore-not-found >/dev/null
kubectl delete cleanuppolicy -n "${TEST_NAMESPACE}" "${CLEANUP_POLICY_NAME}" "${SECOND_CLEANUP_POLICY_NAME}" --ignore-not-found >/dev/null
kubectl delete policyexceptions.kyverno.io -n "${TEST_NAMESPACE}" "${POLEX_NAME}" "${SECOND_POLEX_NAME}" "${DELETE_PROBE_POLEX_NAME}" --ignore-not-found >/dev/null
helm_uninstall_if_present

log "=== scenario B: blocked upgrade, migrate to CEL, then upgrade succeeds ==="

log "B1: fresh 1.19 install, re-applying the legacy fixture"
"${HELM}" install "${RELEASE_NAME}" --repo https://kyverno.github.io/kyverno kyverno \
  --version "${LEGACY_CHART_VERSION}" -n "${NAMESPACE}" --create-namespace --wait --timeout 5m
wait_kyverno_ready
apply_legacy_fixture
wait_for_deny "B1 pre-upgrade ClusterPolicy enforcement" "${WORK_DIR}/violating-pod.yaml" "${CP_DENY_MSG}"

log "B2: snapshotting every controller's deployment image and the release revision before the blocked upgrade attempt"
BEFORE_IMAGES="$(controller_deployment_images)"
BEFORE_REVISION="$(release_revision)"
[ -n "${BEFORE_IMAGES}" ] && [ "${BEFORE_IMAGES}" != "[]" ] && [ "${BEFORE_IMAGES}" != "null" ] \
  || fail "B2: could not read the controller deployment images before the upgrade attempt"
[ -n "${BEFORE_REVISION}" ] || fail "B2: could not read the release revision before the upgrade attempt"

log "B3: attempting the upgrade WITHOUT the opt-out, expecting it to be blocked"
if run_local_upgrade "" "${WORK_DIR}/b3-upgrade.log"; then
  cat "${WORK_DIR}/b3-upgrade.log" >&2
  fail "B3: the upgrade without the opt-out was expected to be blocked"
fi
AFTER_IMAGES="$(controller_deployment_images)"
# Corroborating signal only, not proof on its own: the revision assertion
# below is what actually proves no manifest was applied. This just catches
# a partial apply that left one controller's image changed.
[ "${AFTER_IMAGES}" = "${BEFORE_IMAGES}" ] \
  || fail "B3: a controller deployment image changed even though the upgrade was blocked (before='${BEFORE_IMAGES}' after='${AFTER_IMAGES}')"

# This only leaves a failed/pending-upgrade release recoverable before the
# script asserts below - it does not make the hook-Job block path pass
# those assertions (see the comment above the greps).
STATUS="$(release_status)"
case "${STATUS}" in
  deployed)
    AFTER_REVISION="$(release_revision)"
    # This is the assertion that actually proves no controller manifest
    # was applied before the gate failed: the image check above is only a
    # weaker corroborating signal.
    [ "${AFTER_REVISION}" = "${BEFORE_REVISION}" ] \
      || fail "B3: the release revision advanced (before='${BEFORE_REVISION}' after='${AFTER_REVISION}') even though status is still 'deployed' and the upgrade was blocked - a controller manifest was applied before the gate failed"
    ;;
  failed|pending-upgrade)
    log "B3: blocked upgrade left the release in '${STATUS}' (the pre-upgrade hook Job path, not the render-time gate); recovering to the last deployed revision, then failing below on purpose"
    "${HELM}" rollback "${RELEASE_NAME}" "${BEFORE_REVISION}" -n "${NAMESPACE}" --wait --timeout 5m
    wait_kyverno_ready
    RECOVERED_IMAGES="$(controller_deployment_images)"
    [ "${RECOVERED_IMAGES}" = "${BEFORE_IMAGES}" ] \
      || fail "B3: after recovering from the '${STATUS}' state, the controller deployment images are '${RECOVERED_IMAGES}', expected the pre-upgrade images '${BEFORE_IMAGES}'"
    ;;
  *)
    fail "B3: unexpected release status '${STATUS}' after the blocked upgrade attempt"
    ;;
esac

# These three greps check only the render-time gate's message - the path
# that always fires before the hook Job could run, so it's the only one
# this script exercises (a hook Job's logs never reach this stderr).
grep -q -- "- ClusterPolicy: 1" "${WORK_DIR}/b3-upgrade.log" \
  || { cat "${WORK_DIR}/b3-upgrade.log" >&2; fail "B3: blocked-upgrade error is missing the offending kind/count"; }
grep -qF "${CLUSTERPOLICY_NAME}" "${WORK_DIR}/b3-upgrade.log" \
  || { cat "${WORK_DIR}/b3-upgrade.log" >&2; fail "B3: blocked-upgrade error is missing the offending resource name"; }
grep -q "upgrade.allowLegacyPolicies" "${WORK_DIR}/b3-upgrade.log" \
  || { cat "${WORK_DIR}/b3-upgrade.log" >&2; fail "B3: blocked-upgrade error is missing the upgrade.allowLegacyPolicies opt-out hint"; }
log "B3: PASS (upgrade blocked by the render-time gate with count/name/opt-out hint; no controller manifest change)"

log "B4: migrating to CEL - applying the ValidatingPolicy twin, deleting the legacy ClusterPolicy"
apply_vpol_fixture
kubectl delete clusterpolicy "${CLUSTERPOLICY_NAME}" --ignore-not-found >/dev/null

log "B5: re-running the upgrade without the opt-out, now expecting it to succeed"
run_local_upgrade "" "${WORK_DIR}/b5-upgrade.log" \
  || { cat "${WORK_DIR}/b5-upgrade.log" >&2; fail "B5: the upgrade was expected to succeed once no legacy CRs remain"; }
wait_kyverno_ready

log "B5: verifying the ValidatingPolicy enforces post-upgrade"
wait_for_deny "B5 ValidatingPolicy enforcement post-upgrade" "${WORK_DIR}/violating-pod.yaml" "${VPOL_DENY_MSG}"
log "B5: PASS"

log "=== scenario B: PASS ==="

log "all legacy-policy migration scenarios passed"
