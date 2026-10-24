#!/usr/bin/env bash
#
# verify-legacy-crd-snapshot.sh
#
# Asserts the frozen legacy kyverno.io CRD schemas (#17713) are byte-identical
# to the 1.20 compatibility snapshot taken at the baseline commit recorded in
# config/crds/kyverno/README.md.
#
# Why this exists: once api/kyverno/v1 and v1beta1 stop being generated from
# Go, nothing but this script protects their checked-in CRD bytes. `make
# codegen-crds-kyverno` and `make verify-codegen` only catch *uncommitted*
# drift in the working tree; they say nothing if a PR edits a frozen file (or
# the frozen UpdateRequest v1beta1 block) and commits the result. This script
# compares against inline digests recorded once, at the baseline, so any
# change to those bytes fails CI regardless of whether it was committed.
#
# Written without bash associative arrays (declare -A): macOS ships bash 3.2,
# which does not support them, and this script is meant to run on a laptop as
# readily as in CI.
#
# To intentionally change a frozen schema (a deliberate 1.20 compatibility
# decision, not a routine codegen run): update KYVERNO_CRDS_FROZEN_FILES /
# KYVERNO_CRDS_FROZEN_VERSIONS in the Makefile if the classification itself
# changed, then recompute and paste the new digest(s) below.
#
# Usage (normally invoked via `make verify-legacy-crd-snapshot`, which sets
# these from its own Makefile variables):
#   CRD_SPLICE=.tools/crd-splice CRDS_DIR=config/crds/kyverno \
#     FROZEN_FILES="kyverno.io_clusterpolicies.yaml ..." \
#     FROZEN_VERSIONS="kyverno.io_updaterequests.yaml=v1beta1" \
#     ./scripts/verify-legacy-crd-snapshot.sh

set -euo pipefail

: "${CRD_SPLICE:?CRD_SPLICE must point at the crd-splice binary}"
: "${CRDS_DIR:?CRDS_DIR must point at config/crds/kyverno}"
: "${FROZEN_FILES:?FROZEN_FILES must list the frozen CRD file names}"
: "${FROZEN_VERSIONS:?FROZEN_VERSIONS must list the frozen name=version entries}"

log() { echo "[verify-legacy-crd-snapshot] $*"; }
fail() { echo "[verify-legacy-crd-snapshot] FAIL: $*" >&2; exit 1; }

# Baseline: 22a7827fe (upstream/main, 2026-10-23). Generator:
# hack/controller-gen (sigs.k8s.io/controller-tools v0.22.0, -buildvcs=false,
# hence the "controller-gen.kubebuilder.io/version: (devel)" header), Go
# 1.27.3. File digests are `shasum -a 256 <file>`; the v1beta1 block digest
# is `crd-splice extract ... | shasum -a 256` piped through this script's own
# $(...) capture (which drops the trailing newline before hashing). See
# config/crds/kyverno/README.md for full provenance.
#
# The ClusterPolicy and Policy digests include the imageExtractors[].filter
# field from release-1.19 (#16269), which shipped in 1.19.2 but was never on
# main, so that upgrading from 1.19.2 keeps the field in the schema.
#
# Parallel "name digest" lines rather than associative arrays (bash 3.2 has
# none). file_digest()/version_digest() below do the lookup.
EXPECTED_FILE_DIGESTS='
kyverno.io_clusterpolicies.yaml a97a2e2a0cc7f72faa40343bceacf7a639625a5dfd11dadd2a2e5c03da8f0f9a
kyverno.io_policies.yaml 79258028064bee132b3148bb10a78395d88de51dbfae3635f6983cda50be97c0
kyverno.io_policyexceptions.yaml 58e77a8914e8fbbb60f0df16bedcfaa874fb92a6ba44524d33785a03f5f160f0
kyverno.io_cleanuppolicies.yaml f8cd936675ab43492cb6179c54d0467fca664c3d0cdc34450580364f6a4f6fb6
kyverno.io_clustercleanuppolicies.yaml 5f637228aabd86b6bb1be2379b6deab3e58323671c17f894a4ae162f2914c155
'
EXPECTED_VERSION_DIGESTS='
kyverno.io_updaterequests.yaml=v1beta1 3e0bda892b39a973271fefc2f932b2c0e5dafd3aff99e17d09a25c9a362474dc
'

# lookup prints the digest recorded for $2 in the newline-separated "key
# digest" table $1, or nothing if $2 is not a key.
lookup() {
  printf '%s\n' "$1" | awk -v key="$2" '$1 == key { print $2; found = 1 } END { if (!found) exit 1 }'
}

# keys prints every key (first field) in the newline-separated table $1.
keys() {
  printf '%s\n' "$1" | awk 'NF { print $1 }'
}

FAILED=0

log "checking frozen files against FROZEN_FILES"
for name in ${FROZEN_FILES}; do
  if ! lookup "${EXPECTED_FILE_DIGESTS}" "${name}" > /dev/null; then
    fail "FROZEN_FILES lists ${name} but no digest is recorded for it in this script"
  fi
done
while IFS= read -r name; do
  if [[ " ${FROZEN_FILES} " != *" ${name} "* ]]; then
    fail "a digest is recorded for ${name} but it is not in FROZEN_FILES; the Makefile and this script have drifted"
  fi
  path="${CRDS_DIR}/${name}"
  if [ ! -f "${path}" ]; then
    fail "${path} does not exist"
  fi
  actual="$(shasum -a 256 "${path}" | awk '{print $1}')"
  expected="$(lookup "${EXPECTED_FILE_DIGESTS}" "${name}")"
  if [ "${actual}" = "${expected}" ]; then
    log "  PASS ${name}"
  else
    echo "[verify-legacy-crd-snapshot] FAIL: ${name} digest changed (want ${expected}, got ${actual})" >&2
    FAILED=1
  fi
done <<< "$(keys "${EXPECTED_FILE_DIGESTS}")"

log "checking frozen version entries against FROZEN_VERSIONS"
for entry in ${FROZEN_VERSIONS}; do
  if ! lookup "${EXPECTED_VERSION_DIGESTS}" "${entry}" > /dev/null; then
    fail "FROZEN_VERSIONS lists ${entry} but no digest is recorded for it in this script"
  fi
done
while IFS= read -r entry; do
  if [[ " ${FROZEN_VERSIONS} " != *" ${entry} "* ]]; then
    fail "a digest is recorded for ${entry} but it is not in FROZEN_VERSIONS; the Makefile and this script have drifted"
  fi
  name="${entry%%=*}"
  version="${entry#*=}"
  extracted="$("${CRD_SPLICE}" extract --target "${CRDS_DIR}" --freeze "${name}=${version}")"
  actual="$(printf '%s' "${extracted}" | shasum -a 256 | awk '{print $1}')"
  expected="$(lookup "${EXPECTED_VERSION_DIGESTS}" "${entry}")"
  if [ "${actual}" = "${expected}" ]; then
    log "  PASS ${entry}"
  else
    echo "[verify-legacy-crd-snapshot] FAIL: ${entry} digest changed (want ${expected}, got ${actual})" >&2
    FAILED=1
  fi
  if [ "${name}" = "kyverno.io_updaterequests.yaml" ] && [ "${version}" = "v1beta1" ]; then
    if ! printf '%s\n' "${extracted}" | grep -q '^    served: false$'; then
      fail "the extracted v1beta1 block no longer has served: false"
    fi
    if ! printf '%s\n' "${extracted}" | grep -q '^    deprecated: true$'; then
      fail "the extracted v1beta1 block no longer has deprecated: true"
    fi
    log "  OK   ${entry} still has served: false and deprecated: true"
  fi
done <<< "$(keys "${EXPECTED_VERSION_DIGESTS}")"

if [ "${FAILED}" -ne 0 ]; then
  fail "one or more frozen schemas changed; see above"
fi

log "all frozen legacy CRD schemas match the 1.20 compatibility snapshot"
