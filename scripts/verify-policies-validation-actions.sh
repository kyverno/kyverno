#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${ROOT_DIR}/charts/kyverno-policies"
HELM="${HELM:-helm}"
POLICY="require-non-root-groups"

render_action() {
  local global_action="$1"
  local policy_override="${2:-}"

  local -a helm_args=(
    template kyverno-policies "${CHART_DIR}"
    --set podSecurityStandard=restricted
    --set "includeOtherPolicies={${POLICY}}"
    --set "validationFailureAction=${global_action}"
  )

  if [[ -n "${policy_override}" ]]; then
    helm_args+=(
      --set "validationFailureActionByPolicy.${POLICY}=${policy_override}"
    )
  fi

  "${HELM}" "${helm_args[@]}" | awk -v target="${POLICY}" '
    function emit() {
      if (kind == "ValidatingPolicy" && name == target) {
        print action
        found = 1
      }
    }

    /^---[[:space:]]*$/ {
      emit()
      kind = ""
      name = ""
      action = ""
      next
    }

    /^kind:/ {
      kind = $2
    }

    /^  name:/ {
      name = $2
    }

    /^  validationActions:[[:space:]]*$/ {
      if (getline > 0) {
        gsub(/^[[:space:]]*-[[:space:]]*/, "", $0)
        action = $0
      }
    }

    END {
      emit()
      if (!found) {
        exit 1
      }
    }
  '
}

assert_action() {
  local description="$1"
  local global_action="$2"
  local expected="$3"
  local override="${4:-}"

  local actual
  actual="$(render_action "${global_action}" "${override}")"

  if [[ "${actual}" != "${expected}" ]]; then
    echo "FAIL: ${description}: expected ${expected}, got ${actual}"
    exit 1
  fi

  echo "PASS: ${description} -> ${actual}"
}

assert_action "Global Audit" Audit Audit
assert_action "Global Warn" Warn Warn
assert_action "Global Deny" Deny Deny
assert_action "Legacy Enforce" Enforce Deny
assert_action "Per-policy Deny override" Warn Deny Deny

echo "All validation action checks passed."