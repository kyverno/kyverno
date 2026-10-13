#!/usr/bin/env bash
#
# verify-policies-autogen.sh
#
# Asserts the kyverno-policies chart maps the autogenControllers value to
# spec.autogen.podControllers.controllers on every ValidatingPolicy.
#
# The mapping lives in the kyverno-policies.vpolAutogen helper. The chart CI
# only lints the rendered manifests, so a wrong but valid controller list
# would pass it. This renders the chart and checks the list itself.
#
# This needs no cluster: `helm template` renders locally.
#
# Usage: scripts/verify-policies-autogen.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${ROOT_DIR}/charts/kyverno-policies"
HELM="${HELM:-helm}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

log() { echo "[verify-policies-autogen] $*"; }
fail() { echo "[verify-policies-autogen] FAIL: $*" >&2; exit 1; }

# render prints the chart rendered with the restricted profile, so that the
# baseline and restricted ValidatingPolicy templates are all included.
# The remaining arguments are passed to helm template. Some templates use CRLF
# line endings, so carriage returns are dropped from the output.
render() {
  "${HELM}" template verify "${CHART_DIR}" \
    --set policyType=ValidatingPolicy \
    --set podSecurityStandard=restricted \
    "$@" | tr -d '\r'
}

# values_file writes a values file that sets autogenControllers to $1. A file is
# used because --set would split a list on its commas.
values_file() {
  printf 'autogenControllers: "%s"\n' "$1" >"${TMP_DIR}/values.yaml"
  echo "${TMP_DIR}/values.yaml"
}

# check renders with autogenControllers=$1 (empty means unset) and requires every
# ValidatingPolicy to carry exactly the controllers line $2 (empty means no
# autogen field at all).
check() {
  local value="$1" want="$2" out policies lines
  if [ -n "${value}" ]; then
    out="$(render -f "$(values_file "${value}")")"
  else
    out="$(render)"
  fi
  policies="$(grep -c '^kind: ValidatingPolicy' <<<"${out}" || true)"
  [ "${policies}" -gt 0 ] || fail "no ValidatingPolicy rendered"
  if [ -z "${want}" ]; then
    grep -q 'podControllers' <<<"${out}" && fail "unset: an autogen field was rendered"
    log "  OK   unset: no autogen field on ${policies} policies"
    return
  fi
  lines="$(grep -c "^      controllers: ${want}\$" <<<"${out}" || true)"
  if [ "${lines}" != "${policies}" ]; then
    grep 'controllers:' <<<"${out}" | sort | uniq -c >&2 || true
    fail "${value}: expected ${want} on all ${policies} policies, found it on ${lines}"
  fi
  log "  OK   ${value}: ${want} on ${policies} policies"
}

# check_fails requires that rendering with autogenControllers=$1 fails.
check_fails() {
  if render -f "$(values_file "$1")" >/dev/null 2>&1; then
    fail "autogenControllers=\"$1\" should fail to render"
  fi
  log "  OK   \"$1\" fails to render"
}

log "rendering ${CHART_DIR}"
check "" ""
check "none" "\[\]"
check "Deployment,StatefulSet" '\["deployments","statefulsets"\]'
check "Deployment, Job,CronJob" '\["deployments","jobs","cronjobs"\]'
check_fails "Deploymnt"
check_fails "Deployment,Foo"
check_fails ","
log "all checks passed"
