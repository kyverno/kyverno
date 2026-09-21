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
# Scope flag per CRD_NAMES entry (same positions, same convention as
# count_legacy_crs: "" for cluster-scoped, "-A" for namespaced), so the
# count assertions below can count each legacy kind cluster-wide.
CRD_SCOPES=("" "-A" "-A" "" "-A")
# Display names for the same positions, used by the exclusivity guard's message.
CRD_KINDS=(ClusterPolicy Policy CleanupPolicy ClusterCleanupPolicy PolicyException)

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
# Delete probes: throwaway fixtures deleted in A4b (see that comment) and
# recreated in A5 so the A5 count assertions still match the A2 baseline.
# Excluded only from the per-name spec-hash comparisons.
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
for i in "${!CRD_NAMES[@]}"; do
  PREEXISTING_COUNT="$(count_legacy_crs "${CRD_NAMES[$i]}" "${CRD_SCOPES[$i]}")"
  if [ "${PREEXISTING_COUNT}" != "0" ]; then
    fail "found ${PREEXISTING_COUNT} pre-existing ${CRD_KINDS[$i]} object(s) on this cluster; this script requires exclusive use of the cluster and cannot run correctly alongside them - clear them or use a fresh cluster"
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
    # All three delete-probe names are included below because A5 recreates
    # them; --ignore-not-found tolerates any that somehow aren't present.
    kubectl delete clusterpolicies.kyverno.io "${CLUSTERPOLICY_NAME}" "${SECOND_CLUSTERPOLICY_NAME}" "${DELETE_PROBE_CLUSTERPOLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUP_POLICY_NAME}" "${SECOND_CLUSTERCLEANUP_POLICY_NAME}" "${DELETE_PROBE_CLUSTERCLEANUP_POLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete validatingpolicies.policies.kyverno.io "${VPOL_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    # Namespaced fixtures are also deleted by name, in case the namespace
    # delete above timed out or was already gone.
    kubectl delete policies.kyverno.io -n "${TEST_NAMESPACE}" "${POLICY_NAME}" "${SECOND_POLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete cleanuppolicies.kyverno.io -n "${TEST_NAMESPACE}" "${CLEANUP_POLICY_NAME}" "${SECOND_CLEANUP_POLICY_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete policyexceptions.kyverno.io -n "${TEST_NAMESPACE}" "${POLEX_NAME}" "${SECOND_POLEX_NAME}" "${DELETE_PROBE_POLEX_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete clusterrole "${CLEANUP_RBAC_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    # Whether the uninstall also drops the legacy CRDs is chart-dependent:
    # today they carry no helm.sh/resource-policy, #17494 proposes keep.
    # Either way the guard refused to start unless the cluster was free of
    # legacy CRs, so no CR this run did not create is caught by that cascade.
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

# Emitted rather than applied directly so apply_legacy_fixture can retry
# through wait_for_allow (defined below): applied right after a fresh
# install/rollback, where the webhook may not be serving yet.
legacy_fixture_manifest() {
  cat <<EOF
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

# Applies the primary legacy fixture through wait_for_allow instead of a
# bare apply: right after A1/B1's fresh install, wait_kyverno_ready only
# proves pod readiness, not that the webhook is already serving.
apply_legacy_fixture() {
  local desc="$1"
  legacy_fixture_manifest > "${WORK_DIR}/legacy-fixture.yaml"
  wait_for_allow "${desc}" "${WORK_DIR}/legacy-fixture.yaml"
}

# Emits the throwaway delete-probe fixture (see DELETE_PROBE_CLUSTERPOLICY_NAME).
# Applied in A1, then again in A5 after the rollback.
delete_probe_clusterpolicy_manifest() {
  cat <<EOF
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: ${DELETE_PROBE_CLUSTERPOLICY_NAME}
spec:
  rules: []
EOF
}

# Namespaced twin of legacy_fixture_manifest, as a kyverno.io/v1 Policy.
# Emitted rather than applied so callers can apply it directly or stash it
# to a file for the retry helpers.
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




# CEL twin of legacy_fixture_manifest: a ValidatingPolicy enforcing the
# same require-label rule on Pods in TEST_NAMESPACE. Emitted rather than
# applied directly so apply_vpol_fixture can retry through wait_for_allow.
vpol_fixture_manifest() {
  cat <<EOF
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

# Applies the ValidatingPolicy fixture through wait_for_allow: B4 can run
# right after B3's recovery rollback (see B3), which restarts pods just
# like a fresh install does, so the same webhook-readiness race applies.
apply_vpol_fixture() {
  vpol_fixture_manifest > "${WORK_DIR}/vpol-fixture.yaml"
  wait_for_allow "B4 ValidatingPolicy fixture create" "${WORK_DIR}/vpol-fixture.yaml"
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

# Asserts a `kubectl patch --type merge` is denied for expected_substr. An
# accepted patch fails outright: the create-blocked check above already proved
# the webhook is serving (optional 6th arg: namespace).
wait_for_patch_denied() {
  local desc="$1" resource="$2" name="$3" patch_json="$4" expected_substr="$5" namespace="${6:-}"
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
      fail "${desc}: the spec patch was accepted - the create-blocked assertion above already proved the webhook is serving, so this is the write-time block letting a spec update through, not the webhook still coming up. The fixture is left drifted on purpose; rerun after fixing the block."
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

# Reduces a legacy_webhook_rules_snapshot array (stdin) to the distinct
# (type, config name) pairs that own a legacy-kind rule, one per line as
# "validating<TAB>name" or "mutating<TAB>name". Feeds the reconcile poke.
legacy_webhook_config_targets() {
  jq -r 'map({type, config}) | unique | .[] | "\(.type)\t\(.config)"'
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

# Normalized snapshot of every legacy-kind rule, keyed by type/config/webhook,
# plus the owning webhook's namespaceSelector/objectSelector/failurePolicy/
# matchPolicy. Reads merged config JSON (webhook_configs_json's shape) from
# stdin, so the reconcile-wait loop below can reuse one read for both.
legacy_webhook_rules_from_json() {
  jq -Sc '
      def legacyKinds: ["clusterpolicies","policies","cleanuppolicies","clustercleanuppolicies","policyexceptions"];
      def normSelector(sel):
        (sel // {}) as $s
        | {
            matchLabels: ($s.matchLabels // {}),
            matchExpressions: ($s.matchExpressions // []
              | map({key, operator, values: (.values // [] | sort)})
              | sort_by(.key, .operator))
          };
      [ .[] as $cfg
        | ($cfg.webhooks // [])[] as $wh
        | ($wh.rules // [])[] as $rule
        | select($rule.apiGroups // [] | index("kyverno.io"))
        | (($rule.resources // []) | unique
             | map(select(sub("/\\*$"; "") as $r
                          | legacyKinds | index($r) != null))) as $legacyResources
        | select(($legacyResources | length) > 0)
        | {
            type: $cfg.__whType,
            config: $cfg.metadata.name,
            webhook: $wh.name,
            apiGroups: ($rule.apiGroups // [] | sort),
            apiVersions: ($rule.apiVersions // [] | sort),
            resources: ($legacyResources | sort),
            operations: ($rule.operations // [] | sort),
            scope: ($rule.scope // "*"),
            namespaceSelector: normSelector($wh.namespaceSelector),
            objectSelector: normSelector($wh.objectSelector),
            failurePolicy: ($wh.failurePolicy // "Fail"),
            matchPolicy: ($wh.matchPolicy // "Equivalent")
          }
      ] | sort
    '
}

legacy_webhook_rules_snapshot() {
  webhook_configs_json | legacy_webhook_rules_from_json
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

# Stamps a nonce on every webhook config owning a legacy-kind rule, then
# waits for the controller to overwrite it away: this makes observed differ
# from desired by construction, so its removal proves a write cycle ran.
# baseline_label: a snapshot file if one exists (A4/A5), else "" for a live read.
wait_for_webhook_configs_reconciled() {
  local context="$1" baseline_label="$2"
  local timeout=300 interval=5
  local deadline=$((SECONDS + timeout))
  local targets
  if [ -n "${baseline_label}" ]; then
    targets="$(legacy_webhook_config_targets < "${WORK_DIR}/${baseline_label}-webhook-rules.snapshot")"
    [ -n "${targets}" ] || fail "${context}: found no webhook config owning a legacy-kind rule to poke - the reconcile gate has nothing to prove"
    # A missing mutating target means M2 (the mirrored mutating-webhook rule)
    # is never poked, and nothing else would catch that this gate went vacuous.
    grep -q $'^mutating\t' <<< "${targets}" \
      || fail "${context}: found no MUTATING webhook config owning a legacy-kind rule to poke - mutating coverage would go unverified"
  else
    # No baseline file exists yet for the pre-A2 call, so targets come from a
    # live read - which can itself race the async reconcile this gate exists
    # to wait for, so retry until both kinds show up instead of failing cold.
    while true; do
      targets="$(legacy_webhook_rules_snapshot | legacy_webhook_config_targets || true)"
      if [ -n "${targets}" ] && grep -q $'^mutating\t' <<< "${targets}"; then
        break
      fi
      if [ "${SECONDS}" -ge "${deadline}" ]; then
        fail "${context}: never observed a webhook config owning a legacy-kind rule for both validating and mutating within ${timeout}s - either none exist yet or the API stayed unreachable"
      fi
      sleep "${interval}"
    done
  fi

  local nonce
  nonce="${RUN_ID}-$(date +%s)"
  local wh_type name resource
  while IFS=$'\t' read -r wh_type name; do
    case "${wh_type}" in
      validating) resource=validatingwebhookconfigurations ;;
      mutating) resource=mutatingwebhookconfigurations ;;
      *) fail "${context}: unrecognized webhook type '${wh_type}' in reconcile targets" ;;
    esac
    kubectl annotate --overwrite "${resource}" "${name}" "kyverno.io/legacy-migration-reconcile-probe=${nonce}" >/dev/null \
      || fail "${context}: could not stamp the reconcile probe onto ${resource}/${name}"
  done <<< "${targets}"

  # They heal in parallel, so all targets are stamped above before this
  # single wait loop, rather than waiting on each one in turn.
  local current_json pending have
  while true; do
    # A transient read failure must keep waiting, not abort the script:
    # webhook_configs_json can fail under set -e on an API blip.
    current_json="$(webhook_configs_json || true)"
    if [ -z "${current_json}" ]; then
      pending="<could not read webhook configs>"
    else
      pending=""
      while IFS=$'\t' read -r wh_type name; do
        # Absent is not healed: a config missing from the live cluster must
        # keep failing the wait, not slip through as if it had reconciled.
        have="$(jq -r --arg t "${wh_type}" --arg n "${name}" --arg nonce "${nonce}" '
          [.[] | select(.__whType == $t and .metadata.name == $n)] as $m
          | if ($m | length) == 0 then "absent"
            elif ($m[0].metadata.annotations["kyverno.io/legacy-migration-reconcile-probe"] // "") == $nonce then "pending"
            else "healed" end' <<< "${current_json}" 2>/dev/null || echo "unreadable")"
        [ "${have}" = "healed" ] || pending="${pending}${wh_type}/${name} (${have}); "
      done <<< "${targets}"
    fi
    [ -z "${pending}" ] && return 0
    if [ "${SECONDS}" -ge "${deadline}" ]; then
      fail "${context}: webhook config(s) still carry the reconcile probe after ${timeout}s: ${pending}the controller never rewrote them, so nothing below can prove coverage was preserved - treat as unverified"
    fi
    sleep "${interval}"
  done
}

# Reconcile completion is proven by wait_for_webhook_configs_reconciled
# before this runs, so one read is enough - any diff found here is a real
# regression, not the controller still mid-write.
assert_legacy_webhook_rules_unchanged() {
  local baseline_label="$1" current_label="$2" context="$3"
  local baseline_file="${WORK_DIR}/${baseline_label}-webhook-rules.snapshot"
  local current_file="${WORK_DIR}/${current_label}-webhook-rules.snapshot"
  local current
  current="$(legacy_webhook_rules_snapshot || true)"
  echo "${current}" > "${current_file}"
  if ! diff -u "${baseline_file}" "${current_file}" >"${WORK_DIR}/webhook-rules-diff.log" 2>&1; then
    cat "${WORK_DIR}/webhook-rules-diff.log" >&2
    fail "${context}: legacy-kind webhook rules (validating and mutating) differ from '${baseline_label}' (apiGroups/apiVersions/resources/operations/scope, the owning webhook's namespaceSelector/objectSelector/failurePolicy/matchPolicy, or which type/config/webhook owns them) - see the diff above; an empty current side means the API stayed unreachable"
  fi
}

# Reduces a legacy_webhook_rules_snapshot array to, per legacy kind, the
# sorted set of "type|apiGroups|apiVersion|operation|scope|sub=" capabilities it is
# covered for. A4 tolerates gaining one (e.g. a storage-version fix); only losing one is a regression.
legacy_webhook_capabilities() {
  jq -Sc '
    reduce (.[] | . as $rule | $rule.resources[] as $raw
      | ($raw | sub("/\\*$"; "")) as $kind
      | ($raw | endswith("/*")) as $wildcard
      | $rule.apiVersions[] as $av | $rule.operations[] as $op
      | {kind: $kind,
         triple: ($rule.type + "|" + ($rule.apiGroups | sort | join(",")) + "|" + $av + "|" + $op + "|" + $rule.scope + "|sub=" + ($wildcard | tostring))}) as $e
      ({}; .[$e.kind] += [$e.triple])
    | map_values(unique)
  '
}

# The owning webhook's selector and policy attributes, one entry per
# (type, config, webhook). These have no "wider/narrower" ordering the way
# a capability does, so they are compared for equality, not containment.
legacy_webhook_selectors() {
  jq -Sc '[ .[] | {type, config, webhook, namespaceSelector, objectSelector, failurePolicy, matchPolicy} ] | unique'
}

# Tolerates added capabilities, fails on lost ones (type exact, so a missing
# mutating rule can't hide behind a surviving validating one; selectors and
# failurePolicy compared strictly). One read: wait_for_webhook_configs_reconciled
# already proved reconcile completed, so a failed read here just fails the check.
assert_legacy_webhook_rules_not_narrowed() {
  local baseline_label="$1" current_label="$2" context="$3"
  local baseline_file="${WORK_DIR}/${baseline_label}-webhook-rules.snapshot"
  local current_file="${WORK_DIR}/${current_label}-webhook-rules.snapshot"
  local baseline_caps baseline_selectors
  baseline_caps="$(legacy_webhook_capabilities < "${baseline_file}")"
  baseline_selectors="$(legacy_webhook_selectors < "${baseline_file}")"
  local current
  current="$(legacy_webhook_rules_snapshot || true)"
  if [ -z "${current}" ] || ! jq -e . >/dev/null 2>&1 <<< "${current}"; then
    fail "${context}: could not read legacy-kind webhook rules to compare against '${baseline_label}' - either none exist or the API stayed unreachable"
  fi
  echo "${current}" > "${current_file}"
  local current_caps lost current_selectors selector_diff
  current_caps="$(legacy_webhook_capabilities <<< "${current}")"
  lost="$(jq -n --argjson base "${baseline_caps}" --argjson cur "${current_caps}" '
    def covers($c; $b):
      ($c[0] == $b[0])
      and ($c[1] == $b[1])
      and ($c[2] == $b[2] or $c[2] == "*")
      and ($c[3] == $b[3] or $c[3] == "*")
      and ($c[4] == $b[4] or $c[4] == "*")
      and ($c[5] == $b[5] or $c[5] == "sub=true");
    [ ($base | keys[]) as $k
      | (($cur[$k] // []) | map(split("|"))) as $curTriples
      | ($base[$k] | map(select(. as $t | ($t | split("|")) as $bt
          | ($curTriples | any(covers(.; $bt))) | not))) as $l
      | select(($l | length) > 0)
      | {kind: $k, lost: $l}
    ]')"
  current_selectors="$(legacy_webhook_selectors <<< "${current}")"
  # Subset, not equality: an added webhook for a legacy kind is a widening,
  # which this stage tolerates. Only a baseline entry going missing or
  # changing is a narrowing.
  selector_diff="$(jq -n --argjson base "${baseline_selectors}" --argjson cur "${current_selectors}" \
    '[ $base[] | select(. as $b | ($cur | index([$b]) ) == null) ]' 2>/dev/null || echo "JQ_FAILED")"
  [ "${selector_diff}" = "[]" ] && selector_diff=""
  if [ "${lost}" != "[]" ] || [ -n "${selector_diff}" ]; then
    echo "lost coverage: ${lost}" >&2
    echo "selector/failurePolicy/matchPolicy diff: ${selector_diff}" >&2
    fail "${context}: legacy-kind webhook coverage narrowed from '${baseline_label}' (type|apiGroups|apiVersion|operation|scope|sub triple(s) missing, or a namespaceSelector/objectSelector/failurePolicy/matchPolicy change on the owning webhook) - see above"
  fi
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

# Counts every object of every legacy kind, cluster-wide, so a stray
# duplicate or resurrection is caught even when a per-name spec hash still
# looks untouched. Captures count_legacy_crs's abort explicitly so `set -e` sees it.
legacy_cr_counts_snapshot() {
  local i count
  for i in "${!CRD_NAMES[@]}"; do
    count="$(count_legacy_crs "${CRD_NAMES[$i]}" "${CRD_SCOPES[$i]}")" || return 1
    printf '%s=%s\n' "${CRD_NAMES[$i]}" "${count}"
  done
}

# Compares two legacy_cr_counts_snapshot outputs, per legacy kind at once.
assert_legacy_cr_counts_unchanged() {
  local baseline="$1" current="$2" context="$3"
  [ "${baseline}" = "${current}" ] && return 0
  {
    echo "baseline counts:"
    echo "${baseline}"
    echo "current counts:"
    echo "${current}"
  } >&2
  fail "${context}: legacy CR count(s) changed (see baseline vs current counts above) - a duplicate, deletion, or resurrection of any legacy-kind object would show here even when the fixture's own per-name spec hash still looks unchanged"
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

# Corroborating gate for the webhook-rule assertions below: waits until the
# Deployments' images differ from a pre-change snapshot, so later checks
# start from the post-change manifest (not proof stale pods are gone -
# wait_for_no_stale_controller_pods below is what catches that).
wait_for_controller_images_changed() {
  local desc="$1" before_images="$2"
  local timeout=120 interval=5
  local deadline=$((SECONDS + timeout))
  local current
  while true; do
    current="$(controller_deployment_images)"
    if [ -n "${current}" ] && [ "${current}" != "null" ] && [ "${current}" != "${before_images}" ]; then
      return 0
    fi
    if [ "${SECONDS}" -ge "${deadline}" ]; then
      fail "${desc}: controller deployment images still match the pre-change set after ${timeout}s (before='${before_images}') - the rollout may not have progressed"
    fi
    sleep "${interval}"
  done
}

# Must run before the poke: a terminating old-image pod still reports Ready,
# so it could heal the probe and hide a stale controller. Compares .spec
# images, since .status reports digest-resolved refs that never match the tag.
wait_for_no_stale_controller_pods() {
  local context="$1"
  local timeout=120 interval=5
  local deadline=$((SECONDS + timeout))
  local expected pods_json matched stale
  while true; do
    expected="$(controller_deployment_images)"
    pods_json="$(kubectl get pods -n "${NAMESPACE}" --selector '!job-name' -o json 2>/dev/null || true)"
    # An unreadable expected/pod-list set, or zero pods matching a known
    # component, must NOT read as "no stale pods" - that would pass without
    # having observed anything, the same vacuous-pass failure this guards against.
    if [ -n "${expected}" ] && [ "${expected}" != "null" ] && [ "${expected}" != "[]" ] && [ -n "${pods_json}" ]; then
      matched="$(jq -r --argjson expected "${expected}" '
        (reduce $expected[] as $e ({}; .[$e.component] = $e.image)) as $want
        | [.items[]? | select($want[(.metadata.labels["app.kubernetes.io/component"] // "")] != null)] | length
        ' <<< "${pods_json}" 2>/dev/null || echo 0)"
      if [ "${matched:-0}" -gt 0 ]; then
        stale="$(jq -r --argjson expected "${expected}" '
          (reduce $expected[] as $e ({}; .[$e.component] = $e.image)) as $want
          | .items[]?
          | (.metadata.labels["app.kubernetes.io/component"] // "") as $c
          | select($want[$c] != null)
          | .spec.containers[0].image as $img
          | select($img != $want[$c])
          | "\(.metadata.name)=\($img) (want \($want[$c]))"' <<< "${pods_json}" 2>/dev/null)"
      else
        stale="<no controller pod matched a known component - the read may have failed>"
      fi
    else
      stale="<could not read controller deployment images or the pod list>"
    fi
    [ -z "${stale}" ] && return 0
    if [ "${SECONDS}" -ge "${deadline}" ]; then
      fail "${context}: ${stale//$'\n'/; } after ${timeout}s - could not confirm every controller pod is on the current image, so a stale pod could heal the reconcile-probe poke and mask it"
    fi
    sleep "${interval}"
  done
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
apply_legacy_fixture "A1 legacy ClusterPolicy fixture create"

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
# Policy goes through the same handler as ClusterPolicy and shares the
# same fresh-install webhook race, so it gets the same wait_for_allow
# treatment as apply_legacy_fixture above (no extra RBAC needed either way).
wait_for_allow "A1 policy fixture create" "${WORK_DIR}/policy.yaml"
# wait_for_allow, not a bare apply: RBAC aggregation for the ClusterRole
# just applied is reconciled asynchronously by kube-controller-manager.
wait_for_allow "A1 cleanup policy fixture create" "${WORK_DIR}/cleanup-policy.yaml"
wait_for_allow "A1 cluster cleanup policy fixture create" "${WORK_DIR}/clustercleanup-policy.yaml"
wait_for_allow "A1 policy exception fixture create" "${WORK_DIR}/polex.yaml"

log "A1: applying dedicated delete-probe fixtures (ClusterPolicy, ClusterCleanupPolicy, PolicyException) - these exist only to be deleted in A4b while the 1.20 write-block is active, and recreated in A5 after the rollback (excluded only from the per-name spec-hash assertions)"
clustercleanuppolicy_manifest "${DELETE_PROBE_CLUSTERCLEANUP_POLICY_NAME}" > "${WORK_DIR}/delete-probe-clustercleanup-policy.yaml"
polex_manifest "${DELETE_PROBE_POLEX_NAME}" > "${WORK_DIR}/delete-probe-polex.yaml"
delete_probe_clusterpolicy_manifest > "${WORK_DIR}/delete-probe-clusterpolicy.yaml"
wait_for_allow "A1 delete-probe ClusterPolicy fixture create" "${WORK_DIR}/delete-probe-clusterpolicy.yaml"
# wait_for_allow, not a bare apply: same RBAC-aggregation race as the
# ClusterCleanupPolicy/PolicyException fixtures above.
wait_for_allow "A1 delete-probe cluster cleanup policy fixture create" "${WORK_DIR}/delete-probe-clustercleanup-policy.yaml"
wait_for_allow "A1 delete-probe policy exception fixture create" "${WORK_DIR}/delete-probe-polex.yaml"

log "A1: verifying the legacy policies enforce on 1.19 (violating pod denied by both fixtures, compliant pod admitted)"
wait_for_deny "A1 pre-upgrade ClusterPolicy enforcement" "${WORK_DIR}/violating-pod.yaml" "${CP_DENY_MSG}"
wait_for_deny "A1 pre-upgrade Policy enforcement" "${WORK_DIR}/violating-pod.yaml" "${POLICY_DENY_MSG}"
wait_for_allow "A1 pre-upgrade compliant pod" "${WORK_DIR}/compliant-pod.yaml"
kubectl delete -f "${WORK_DIR}/compliant-pod.yaml" --ignore-not-found >/dev/null

log "A2: snapshotting the baseline (CRD versions/spec hash/storedVersions, per-kind spec hashes, legacy CR counts across all five kinds, webhook rules)"
snapshot_crds "baseline"
BASELINE_CLUSTERPOLICY_HASH="$(clusterpolicy_spec_hash "${CLUSTERPOLICY_NAME}")"
BASELINE_CR_COUNTS="$(legacy_cr_counts_snapshot)"
BASELINE_POLICY_HASH="$(namespaced_spec_hash policies.kyverno.io "${POLICY_NAME}")"
BASELINE_CLEANUP_POLICY_HASH="$(namespaced_spec_hash cleanuppolicy "${CLEANUP_POLICY_NAME}")"
BASELINE_CLUSTERCLEANUP_POLICY_HASH="$(clusterscoped_spec_hash clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUP_POLICY_NAME}")"
BASELINE_POLEX_HASH="$(namespaced_spec_hash policyexceptions.kyverno.io "${POLEX_NAME}")"
webhook_rules_cover_legacy_kinds "A2 baseline"
# No baseline file exists yet, so this call derives its poke targets from a
# live read (baseline_label "") instead of the usual snapshot file.
wait_for_webhook_configs_reconciled "A2 baseline" ""
snapshot_legacy_webhook_rules "baseline"

log "A3: upgrading to the LOCAL chart with upgrade.allowLegacyPolicies=true (default write-block left ON)"
PRE_A3_IMAGES="$(controller_deployment_images)"
[ -n "${PRE_A3_IMAGES}" ] && [ "${PRE_A3_IMAGES}" != "[]" ] && [ "${PRE_A3_IMAGES}" != "null" ] \
  || fail "A3: could not read the controller deployment images before the change, so the image gate below cannot tell a real rollout from a failed read"
run_local_upgrade "--set upgrade.allowLegacyPolicies=true" "${WORK_DIR}/a3-upgrade.log" \
  || { cat "${WORK_DIR}/a3-upgrade.log" >&2; fail "A3: opt-out upgrade to the local chart was expected to succeed"; }
wait_for_controller_images_changed "A3" "${PRE_A3_IMAGES}"
wait_kyverno_ready

log "A4: post-upgrade assertions"
# Before anything is asserted: a surviving 1.19 pod would satisfy the
# enforcement checks below, proving the old build still works rather than
# the new one.
wait_for_no_stale_controller_pods "A4"
POST_UPGRADE_CLUSTERPOLICY_HASH="$(clusterpolicy_spec_hash "${CLUSTERPOLICY_NAME}")"
if [ "${POST_UPGRADE_CLUSTERPOLICY_HASH}" != "${BASELINE_CLUSTERPOLICY_HASH}" ]; then
  fail "A4: the pre-existing ClusterPolicy's spec changed across the opt-out upgrade"
fi
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

SPEC_PATCH='{"spec":{"rules":[{"name":"check-label","match":{"resources":{"kinds":["Pod"],"namespaces":["'"${TEST_NAMESPACE}"'"]}},"validate":{"message":"changed","pattern":{"metadata":{"labels":{"app":"?*"}}}}}]}}'
wait_for_patch_denied "A4 spec-update blocked" "clusterpolicy" "${CLUSTERPOLICY_NAME}" "${SPEC_PATCH}" "no longer accepted"

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

POLICY_SPEC_PATCH='{"spec":{"rules":[{"name":"check-label","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"changed","pattern":{"metadata":{"labels":{"app":"?*"}}}}}]}}'
wait_for_patch_denied "A4 policy spec-update blocked" "policies.kyverno.io" "${POLICY_NAME}" "${POLICY_SPEC_PATCH}" "no longer accepted" "${TEST_NAMESPACE}"

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

CLEANUP_SPEC_PATCH='{"spec":{"schedule":"0 0 2 1 *"}}'
wait_for_patch_denied "A4 cleanup policy spec-update blocked" "cleanuppolicy" "${CLEANUP_POLICY_NAME}" "${CLEANUP_SPEC_PATCH}" "no longer accepted" "${TEST_NAMESPACE}"
CLUSTERCLEANUP_SPEC_PATCH='{"spec":{"schedule":"0 0 2 1 *"}}'
wait_for_patch_denied "A4 cluster cleanup policy spec-update blocked" "clustercleanuppolicies.kyverno.io" "${CLUSTERCLEANUP_POLICY_NAME}" "${CLUSTERCLEANUP_SPEC_PATCH}" "no longer accepted"
POLEX_SPEC_PATCH='{"spec":{"exceptions":[{"policyName":"legacy-policy-migration-verify-changed","ruleNames":["*"]}]}}'
wait_for_patch_denied "A4 policy exception spec-update blocked" "policyexceptions.kyverno.io" "${POLEX_NAME}" "${POLEX_SPEC_PATCH}" "no longer accepted" "${TEST_NAMESPACE}"

wait_for_annotate_allowed "A4 cleanup policy metadata patch allowed" "cleanuppolicy" "${CLEANUP_POLICY_NAME}" "legacy-policy-migration-verify/probe=1" "${TEST_NAMESPACE}"
wait_for_annotate_allowed "A4 cluster cleanup policy metadata patch allowed" "clustercleanuppolicies.kyverno.io" "${CLUSTERCLEANUP_POLICY_NAME}" "legacy-policy-migration-verify/probe=1"
wait_for_annotate_allowed "A4 policy exception metadata patch allowed" "policyexceptions.kyverno.io" "${POLEX_NAME}" "legacy-policy-migration-verify/probe=1" "${TEST_NAMESPACE}"

# Sampled here, not right after the upgrade: every create above is expected
# to be denied and wait_for_deny force-deletes any spurious success, so
# nothing in this block can grow the count - but sampling this late still
# catches a controller-driven resurrection the early read would miss.
POST_UPGRADE_CR_COUNTS="$(legacy_cr_counts_snapshot)"
assert_legacy_cr_counts_unchanged "${BASELINE_CR_COUNTS}" "${POST_UPGRADE_CR_COUNTS}" "A4"

# Second, late hash sample per kind, vs BASELINE: catches spec drift that
# lands during the block, which an early-only sample would miss.
LATE_CLUSTERPOLICY_HASH="$(clusterpolicy_spec_hash "${CLUSTERPOLICY_NAME}")"
[ "${LATE_CLUSTERPOLICY_HASH}" = "${BASELINE_CLUSTERPOLICY_HASH}" ] \
  || fail "A4: the ClusterPolicy's spec drifted from baseline during the A4 block (late sample)"
LATE_POLICY_HASH="$(namespaced_spec_hash policies.kyverno.io "${POLICY_NAME}")"
[ "${LATE_POLICY_HASH}" = "${BASELINE_POLICY_HASH}" ] \
  || fail "A4: the Policy's spec drifted from baseline during the A4 block (late sample)"
LATE_CLEANUP_POLICY_HASH="$(namespaced_spec_hash cleanuppolicy "${CLEANUP_POLICY_NAME}")"
[ "${LATE_CLEANUP_POLICY_HASH}" = "${BASELINE_CLEANUP_POLICY_HASH}" ] \
  || fail "A4: the CleanupPolicy's spec drifted from baseline during the A4 block (late sample)"
LATE_CLUSTERCLEANUP_POLICY_HASH="$(clusterscoped_spec_hash clustercleanuppolicies.kyverno.io "${CLUSTERCLEANUP_POLICY_NAME}")"
[ "${LATE_CLUSTERCLEANUP_POLICY_HASH}" = "${BASELINE_CLUSTERCLEANUP_POLICY_HASH}" ] \
  || fail "A4: the ClusterCleanupPolicy's spec drifted from baseline during the A4 block (late sample)"
LATE_POLEX_HASH="$(namespaced_spec_hash policyexceptions.kyverno.io "${POLEX_NAME}")"
[ "${LATE_POLEX_HASH}" = "${BASELINE_POLEX_HASH}" ] \
  || fail "A4: the PolicyException's spec drifted from baseline during the A4 block (late sample)"

snapshot_crds "post-upgrade"
# Must stay after the whole A4 block above (not just after the upgrade):
# that's what gives it the same implicit settle window the CR-count and
# spec-hash late samples get explicitly. Do not reorder it earlier.
assert_crds_unchanged "baseline" "post-upgrade" "A4"
wait_for_webhook_configs_reconciled "A4" "baseline"
# Not strict equality: baseline is the published 1.19 chart and this is
# the local chart, so a legitimate capability addition must not fail this.
assert_legacy_webhook_rules_not_narrowed "baseline" "post-upgrade" "A4"
log "A4: PASS (enforcement intact; create/spec-update blocked and metadata patch allowed on all five legacy kinds across all three block handlers; CR counts and per-kind spec hashes late-sampled against baseline, CRDs unchanged, webhook coverage not narrowed, and no owning-webhook selector/failurePolicy/matchPolicy change)"

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
PRE_A5_IMAGES="$(controller_deployment_images)"
[ -n "${PRE_A5_IMAGES}" ] && [ "${PRE_A5_IMAGES}" != "[]" ] && [ "${PRE_A5_IMAGES}" != "null" ] \
  || fail "A5: could not read the controller deployment images before the change, so the image gate below cannot tell a real rollout from a failed read"
"${HELM}" rollback "${RELEASE_NAME}" 1 -n "${NAMESPACE}" --wait --timeout 5m
wait_for_controller_images_changed "A5" "${PRE_A5_IMAGES}"
wait_kyverno_ready
# Before anything is asserted or recreated: a surviving 1.20 pod would
# satisfy the checks below on the build we just rolled away from.
wait_for_no_stale_controller_pods "A5"

# Confirm all three A4b delete probes are genuinely absent: a blind
# re-apply would mask a resurrected probe. Must run before any recreation
# below, or the recreation's own idempotent apply would hide that.
assert_delete_probe_absent "A5" "${DELETE_PROBE_CLUSTERPOLICY_NAME}" clusterpolicy
assert_delete_probe_absent "A5" "${DELETE_PROBE_CLUSTERCLEANUP_POLICY_NAME}" clustercleanuppolicies.kyverno.io
assert_delete_probe_absent "A5" "${DELETE_PROBE_POLEX_NAME}" policyexceptions.kyverno.io -n "${TEST_NAMESPACE}"

# Recreate all three now that 1.19 unblocks creates again (couldn't be done
# any earlier). All three are counted in the A2 baseline, so without this
# the post-rollback count check below would see baseline-minus-one and fail.
log "A5: recreating the three delete-probes deleted in A4b, so the count assertion below still matches the untouched baseline"
# wait_for_allow, not bare applies: the rollback just restarted the
# admission and cleanup controllers, so their webhooks may briefly reject
# these creates before they are ready.
wait_for_allow "A5 delete-probe ClusterPolicy recreate" "${WORK_DIR}/delete-probe-clusterpolicy.yaml"
wait_for_allow "A5 delete-probe cluster cleanup policy recreate" "${WORK_DIR}/delete-probe-clustercleanup-policy.yaml"
wait_for_allow "A5 delete-probe policy exception recreate" "${WORK_DIR}/delete-probe-polex.yaml"

log "A5: post-rollback assertions"
POST_ROLLBACK_CLUSTERPOLICY_HASH="$(clusterpolicy_spec_hash "${CLUSTERPOLICY_NAME}")"
if [ "${POST_ROLLBACK_CLUSTERPOLICY_HASH}" != "${POST_UPGRADE_CLUSTERPOLICY_HASH}" ]; then
  fail "A5: the ClusterPolicy's spec changed across the rollback (it should carry over the A4 metadata annotation untouched, and the spec itself must be identical)"
fi
POST_ROLLBACK_CR_COUNTS="$(legacy_cr_counts_snapshot)"
assert_legacy_cr_counts_unchanged "${BASELINE_CR_COUNTS}" "${POST_ROLLBACK_CR_COUNTS}" "A5"
wait_for_deny "A5 ClusterPolicy enforcement still active post-rollback" "${WORK_DIR}/violating-pod.yaml" "${CP_DENY_MSG}"
wait_for_deny "A5 Policy enforcement still active post-rollback" "${WORK_DIR}/violating-pod.yaml" "${POLICY_DENY_MSG}"
snapshot_crds "post-rollback"
assert_crds_unchanged "baseline" "post-rollback" "A5"
wait_for_webhook_configs_reconciled "A5" "baseline"
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
log "A5: PASS (CR counts across all five legacy kinds and per-fixture spec unchanged; CRDs, webhook rules, and enforcement unchanged; write-time block itself rolled back in both controllers)"

log "=== scenario A: PASS ==="

# ==============================================================================
# Reset between scenarios
# ==============================================================================

log "=== resetting for scenario B ==="
# All three delete-probe names are included below because A5 recreated all
# of them (see the comment there); --ignore-not-found tolerates any that
# somehow aren't present.
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
apply_legacy_fixture "B1 legacy ClusterPolicy fixture create"
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
PRE_B5_IMAGES="$(controller_deployment_images)"
[ -n "${PRE_B5_IMAGES}" ] && [ "${PRE_B5_IMAGES}" != "[]" ] && [ "${PRE_B5_IMAGES}" != "null" ] \
  || fail "B5: could not read the controller deployment images before the change, so the image gate below cannot tell a real rollout from a failed read"
run_local_upgrade "" "${WORK_DIR}/b5-upgrade.log" \
  || { cat "${WORK_DIR}/b5-upgrade.log" >&2; fail "B5: the upgrade was expected to succeed once no legacy CRs remain"; }
wait_for_controller_images_changed "B5" "${PRE_B5_IMAGES}"
wait_kyverno_ready
# Before anything is asserted: release-1.19 ships the ValidatingPolicy CRDs
# too, so a terminating-but-Ready pre-upgrade pod could serve this deny and
# mask a broken new build.
wait_for_no_stale_controller_pods "B5"

log "B5: verifying the ValidatingPolicy enforces post-upgrade"
wait_for_deny "B5 ValidatingPolicy enforcement post-upgrade" "${WORK_DIR}/violating-pod.yaml" "${VPOL_DENY_MSG}"
log "B5: PASS"

log "=== scenario B: PASS ==="

log "all legacy-policy migration scenarios passed"
