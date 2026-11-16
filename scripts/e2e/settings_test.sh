#!/usr/bin/env bash
# Offline tests for scripts/e2e/settings.sh, which make test-e2e-local uses.
#
# Most cases run against the small workflow copies in testdata/settings. The
# last case runs every suite of the real workflows and checks that no setting
# keeps an unresolved GitHub expression.
#
# Usage: scripts/e2e/settings_test.sh (requires yq and jq)
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
settings="$here/settings.sh"
fixtures="$here/testdata/settings"
failures=0

# fixture <arguments...> runs settings.sh against the fixtures.
fixture() {
  E2E_WORKFLOW="$fixtures/tests-conformance.yaml" E2E_ACTION="$fixtures/action.yaml" \
    E2E_MATRIX="$fixtures/kubernetes-matrix.yaml" "$settings" "$@"
}

# expect <description> <expected> <actual>
expect() {
  if [ "$2" == "$3" ]; then
    echo "ok: $1"
  else
    echo "FAIL: $1"
    echo "  expected: $2"
    echo "  actual:   $3"
    failures=$((failures + 1))
  fi
}

# expect_error <description> <message part> <arguments...> checks that
# settings.sh fails with a message that contains <message part>.
expect_error() {
  local description="$1" part="$2" output
  shift 2
  if output="$(fixture "$@" 2>&1)"; then
    expect "$description fails" "exit status 1" "exit status 0"
  elif [[ "$output" == *"$part"* ]]; then
    echo "ok: $description"
  else
    expect "$description" "message with '$part'" "$output"
  fi
}

# Input defaults come from the action, the latest version from the matrix.
expect "defaults" "k8s-version=v1.36.3
kind-image=kindest/node:v1.36.3@sha256:cccc
kind-version=v0.99.0
kind-config=default
kyverno-configs=standard
explicit-install-settings=
install-cert-manager=false
install-openreports=false
install-kubectl-evict=false
variant=
job=plain" "$(fixture plain)"
expect "version by minor" "kindest/node:v1.34.1@sha256:aaaa" "$(fixture plain 1.34 kind-image)"
expect "version with v prefix" "v1.35.2" "$(fixture plain v1.35 k8s-version)"
expect "full version" "v1.35.2" "$(fixture plain v1.35.2 k8s-version)"

# Suites limited to the beta or stable MutatingAdmissionPolicy versions.
expect "beta suite defaults to the latest beta version" "v1.35.2" "$(fixture beta '' k8s-version)"
expect "beta suite settings" "vap-v1beta1 standard,beta" \
  "$(fixture beta '' kind-config) $(fixture beta '' kyverno-configs)"
expect "stable suite defaults to the latest stable version" "v1.36.3" "$(fixture stable '' k8s-version)"
expect "boolean input" "true" "$(fixture stable '' install-kubectl-evict)"
expect "untested version runs with a warning" \
  "settings.sh: CI doesn't run beta on v1.36.3; running it anyway." "$(fixture beta 1.36 2>&1 > /dev/null)"

# Certificate jobs: one suite per key-algorithm entry.
expect "certificate matrix expansion" "--set certManager.algorithm=ECDSA --set certManager.size=256" \
  "$(fixture tls/ecdsa-cert-manager '' explicit-install-settings)"
expect "certificate install" "true certs" \
  "$(fixture tls/rsa-cert-manager '' install-cert-manager) $(fixture tls/rsa-cert-manager '' job)"

# Suites with several variants.
expect "rbac defaults to the first variant" "standard standard" \
  "$(fixture rbac '' kyverno-configs) $(fixture rbac '' variant)"
expect "rbac VARIANT selection" "default default" \
  "$(VARIANT=default fixture rbac '' kyverno-configs) $(VARIANT=default fixture rbac '' variant)"

# Invalid arguments.
expect_error "missing suite" "set SUITE to one of: beta plain rbac stable tls/ecdsa-cert-manager tls/rsa-cert-manager" ""
expect_error "unknown suite" "no conformance job runs 'nope'" nope
expect_error "unknown version" "no node image for Kubernetes 1.30. Use one of: v1.34.1 v1.35.2 v1.36.3" plain 1.30
expect_error "unknown key" "unknown key nope" plain '' nope
VARIANT=nope expect_error "unknown variant" "CI doesn't run 'rbac' with variant 'nope'. Use one of: standard default" rbac

# No setting of any real suite or variant keeps a GitHub expression.
# The suite list comes from the error message for a missing suite, so it
# includes the expanded certificate suites. Each rbac variant runs too.
list="$("$settings" 2>&1 || true)"
read -r -a suites <<< "${list#*set SUITE to one of: }"
unresolved=""
checked=0
for entry in "${suites[@]}" rbac:default rbac:force-failure-policy-ignore; do
  suite="${entry%%:*}"
  variant=""
  if [[ "$entry" == *:* ]]; then variant="${entry#*:}"; fi
  checked=$((checked + 1))
  # shellcheck disable=SC2016 # Match a literal GitHub expression.
  if ! output="$(VARIANT="$variant" "$settings" "$suite" 2>&1)" || [[ "$output" == *'${{'* ]]; then
    unresolved+="$entry: $output; "
  fi
done
expect "real workflows resolve every setting (more than 30 suites and variants)" "true" \
  "$([ "$checked" -gt 30 ] && echo true || echo "false, only $checked")"
expect "real workflows have no unresolved expression" "" "$unresolved"

if [ "$failures" -gt 0 ]; then
  echo "$failures settings.sh checks failed."
  exit 1
fi
echo "All settings.sh checks passed."
