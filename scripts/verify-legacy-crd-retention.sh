#!/usr/bin/env bash
#
# verify-legacy-crd-retention.sh
#
# Asserts the #17489 legacy-CRD retention annotation
# (helm.sh/resource-policy: keep) is present on exactly the five legacy
# kyverno.io policy CRDs in a rendered chart, and on nothing else.
#
# Why this exists: the annotation originates from a kubebuilder marker on
# the Go types and reaches the chart through codegen, so nothing in the
# chart itself pins it. A future codegen change, a marker dropped during
# the 1.21 legacy-API removal work, or a hand-edit to the CRD templates can
# silently remove it. Losing it re-opens the data-loss path this annotation
# exists to close: a Helm uninstall would cascade-delete the live legacy
# CRDs and every policy resource stored under them.
#
# It renders the chart with `helm template` and checks three things:
#
#   1. each of the five legacy CRDs carries the annotation
#   2. no other rendered CRD carries it (exactly five in total)
#   3. the new policies.kyverno.io PolicyException specifically does not
#
# Check 3 is called out separately because two different CRDs are named
# "policyexceptions": the legacy kyverno.io one, which must be kept, and
# the new policyexceptions.policies.kyverno.io, which must not be. For the
# same reason every name comparison below is an exact string match on
# .metadata.name rather than a grep over the render: the legacy Policy CRD
# is literally named "policies.kyverno.io", which is a substring of every
# CRD name in the new policies.kyverno.io API group
# (validatingpolicies.policies.kyverno.io and friends), so a substring
# match would report the new group's CRDs as legacy ones.
#
# This needs no cluster: `helm template` renders locally.
#
# Usage: scripts/verify-legacy-crd-retention.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${ROOT_DIR}/charts/kyverno"
RELEASE_NAME="legacy-crd-retention-verify"
# Honor a pinned helm binary passed in by the Makefile (HELM=$(HELM), the
# .tools/helm the rest of the build uses), falling back to whatever "helm"
# resolves to on PATH when run standalone.
HELM="${HELM:-helm}"
# `helm template` has no cluster to ask, so it falls back to Helm's own
# built-in default Capabilities.KubeVersion, which can be older than this
# chart's `kubeVersion` constraint. Pin it the way the Makefile's other
# `helm template --kube-version $(KUBE_VERSION)` calls do.
KUBE_VERSION="${KUBE_VERSION:-v1.25.0}"

ANNOTATION="helm.sh/resource-policy: keep"

# The five legacy kyverno.io policy CRDs from #17489. Note "policies.kyverno.io"
# here is the legacy Policy CRD, NOT the new API group of the same name.
EXPECTED_KEPT=(
  clusterpolicies.kyverno.io
  policies.kyverno.io
  cleanuppolicies.kyverno.io
  clustercleanuppolicies.kyverno.io
  policyexceptions.kyverno.io
)

# CRDs that must NOT be kept, asserted by name on top of the exact-count
# check below. The new-API PolicyException is the one a regression is most
# likely to catch by mistake, since it shares a plural with the legacy kind.
EXPECTED_NOT_KEPT=(
  policyexceptions.policies.kyverno.io
  validatingpolicies.policies.kyverno.io
  updaterequests.kyverno.io
  globalcontextentries.kyverno.io
)

log() { echo "[verify-legacy-crd-retention] $*"; }
fail() { echo "[verify-legacy-crd-retention] FAIL: $*" >&2; exit 1; }

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

RENDER="${WORK_DIR}/render.yaml"
log "rendering ${CHART_DIR} with --kube-version ${KUBE_VERSION}"
"${HELM}" template "${RELEASE_NAME}" "${CHART_DIR}" \
  --kube-version "${KUBE_VERSION}" \
  --set crds.install=true \
  > "${RENDER}"

# Build "<crd name> <kept|none>" for every CustomResourceDefinition in the
# render. Documents are split on the YAML "---" separator. Within a
# document, the first line matching "^  name: " is .metadata.name: metadata
# precedes spec in every rendered CRD, and the only other two-space-indented
# "name:" keys in a CRD live under spec.names/spec.versions, which come
# later. Everything in the OpenAPI schema is indented far deeper.
awk '
  BEGIN { RS = "\n---\n" }
  {
    if ($0 !~ /(^|\n)kind: CustomResourceDefinition(\n|$)/) next
    name = ""; kept = "none"
    n = split($0, lines, "\n")
    for (i = 1; i <= n; i++) {
      if (name == "" && lines[i] ~ /^  name: /) { name = lines[i]; sub(/^  name: /, "", name) }
      if (lines[i] ~ /^    helm\.sh\/resource-policy: keep[[:space:]]*$/) kept = "kept"
    }
    if (name != "") print name, kept
  }
' "${RENDER}" | sort > "${WORK_DIR}/crds.txt"

TOTAL_CRDS="$(wc -l < "${WORK_DIR}/crds.txt" | tr -d ' ')"
if [ "${TOTAL_CRDS}" -eq 0 ]; then
  fail "the chart render contained no CustomResourceDefinition documents; is crds.install still honored, or did the render layout change? (render kept at ${RENDER})"
fi
log "found ${TOTAL_CRDS} CRDs in the render"

# kept_state echoes "kept", "none", or "absent" for one CRD name, matching
# on the whole field rather than a substring (see the header comment on the
# policies.kyverno.io name collision).
kept_state() {
  awk -v want="$1" '$1 == want { print $2; found = 1 } END { if (!found) print "absent" }' "${WORK_DIR}/crds.txt"
}

log "checking the five legacy CRDs carry '${ANNOTATION}'"
for name in "${EXPECTED_KEPT[@]}"; do
  state="$(kept_state "${name}")"
  case "${state}" in
    kept) log "  OK   ${name}" ;;
    none) fail "legacy CRD ${name} is missing the '${ANNOTATION}' annotation. A Helm uninstall would cascade-delete it and every policy resource stored under it. The annotation comes from a +kubebuilder:metadata:annotations marker on the Go type - check api/kyverno/*/ for the marker on every served version of this kind, then re-run 'make codegen-crds-all codegen-helm-crds'." ;;
    absent) fail "legacy CRD ${name} was not present in the chart render at all" ;;
  esac
done

log "checking nothing else carries it"
KEPT_COUNT="$(awk '$2 == "kept"' "${WORK_DIR}/crds.txt" | wc -l | tr -d ' ')"
if [ "${KEPT_COUNT}" -ne "${#EXPECTED_KEPT[@]}" ]; then
  awk '$2 == "kept" { print "  " $1 }' "${WORK_DIR}/crds.txt" >&2
  fail "expected exactly ${#EXPECTED_KEPT[@]} CRDs to carry '${ANNOTATION}', found ${KEPT_COUNT} (listed above). Retention is deliberately limited to the five legacy kyverno.io policy CRDs; a new one here means a marker landed on a type that should not have it."
fi

for name in "${EXPECTED_NOT_KEPT[@]}"; do
  state="$(kept_state "${name}")"
  case "${state}" in
    none) log "  OK   ${name} (correctly not kept)" ;;
    kept) fail "${name} carries '${ANNOTATION}' but must not: it is not one of the five legacy policy CRDs, so Helm should be free to prune it normally." ;;
    absent) fail "expected ${name} in the chart render, but it was absent; update this script if the chart's CRD set changed" ;;
  esac
done

log "legacy CRD retention verified: exactly the five legacy kyverno.io policy CRDs carry '${ANNOTATION}'"
