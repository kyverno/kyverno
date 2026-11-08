#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${ROOT_DIR}/charts/kyverno-policies"
HELM="${HELM:-helm}"

rendered="$("${HELM}" template kyverno-policies "${CHART_DIR}" \
  --set podSecurityStandard=restricted \
  --set 'includeOtherPolicies={require-non-root-groups}' \
  --set validationFailureAction=Warn \
  --set validationFailureActionByPolicy.require-non-root-groups=Deny)"

policy_action="$(printf '%s\n' "$rendered" | awk '
  /^kind: ValidatingPolicy$/ { in_policy=1; name=""; action=""; next }
  /^---$/ {
    if (name == "require-non-root-groups") {
      print action
      exit
    }
    in_policy=0
  }
  in_policy && /^  name:/ { name=$2 }
  in_policy && /validationActions:/ {
    getline
    action=$2
  }
')"

if [[ "${policy_action}" != "Deny" ]]; then
  echo "FAIL: expected per-policy Deny override to render as Deny, got '${policy_action}'"
  exit 1
fi

echo "OK: per-policy Deny override renders as Deny"