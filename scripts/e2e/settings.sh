#!/usr/bin/env bash
# Prints the settings that CI uses to run a conformance suite, so that
# `make test-e2e-local` runs it the same way. It reads them from the workflows,
# so it follows changes to the suites and to the Kubernetes matrix:
#   - .github/workflows/tests-conformance.yaml: the job that runs the suite.
#   - .github/actions/tests/conformance/run/action.yaml: the input defaults.
#   - .github/workflows/kubernetes-matrix.yaml: the node images and kind version.
#
# Usage: settings.sh <suite> [<k8s-version>] [<key>]
#
# <suite> is the tests-path of the job, a directory under
# test/conformance/chainsaw, for example validating-policies or
# tls-certificates/rsa-cert-manager. <k8s-version> is a version from the
# matrix, such as v1.36, 1.36, or v1.36.4. It defaults to the latest version
# that the job tests. Without <key>, the script prints every setting as
# key=value. With <key>, it prints only that value.
#
# For a suite that CI runs with several settings, such as rbac with three
# kyverno-configs values, set VARIANT to the variant of one job. The default
# is the first one.
#
# Requires yq (https://github.com/mikefarah/yq) and jq.
set -euo pipefail

suite="${1:-}"
k8s="${2:-}"
key="${3:-}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# The tests in settings_test.sh point these at fixtures.
workflow="${E2E_WORKFLOW:-$root/.github/workflows/tests-conformance.yaml}"
action="${E2E_ACTION:-$root/.github/actions/tests/conformance/run/action.yaml}"
matrix="${E2E_MATRIX:-$root/.github/workflows/kubernetes-matrix.yaml}"

fail() {
  echo "settings.sh: $*" >&2
  exit 1
}

for tool in yq jq; do
  command -v "$tool" > /dev/null || fail "$tool is required"
done

# Every use of the conformance action, one JSON object per line and per
# combination of the matrix dimensions other than k8s-version and shard-index,
# with the ${{ matrix.* }} expressions of those dimensions expanded.
jobs="$(yq -o=json '.jobs' "$workflow" | jq -c '
  to_entries[] | .key as $job | .value as $spec
  | .value.steps[]? | select(.uses == "./.github/actions/tests/conformance/run") | .with as $with
  | [($spec.strategy.matrix // {}) | to_entries[]
     | select(.key != "k8s-version" and .key != "shard-index" and (.value | type) == "array")] as $dims
  | (reduce $dims[] as $d ([{}]; [.[] as $c | $d.value[] as $v | $c + {($d.key): $v}]))[] as $combo
  | {job: $job, versions: $spec.strategy.matrix["k8s-version"],
     with: ($with | with_entries(.value |= (if type == "string" then
       gsub("\\$\\{\\{ matrix\\.(?<k>[a-z-]+)(\\.(?<f>[a-z-]+))? \\}\\}";
         . as $m | ($combo[$m.k] // "") | (if $m.f then .[$m.f] // "" else . end) | tostring)
       else . end)))}')"

suites() {
  jq -r '.with["tests-path"]' <<< "$jobs" | sort -u | tr '\n' ' '
}

[ -n "$suite" ] || fail "set SUITE to one of: $(suites)"
candidates="$(jq -c --arg suite "$suite" 'select(.with["tests-path"] == $suite)' <<< "$jobs")"
[ -n "$candidates" ] || fail "no conformance job runs '$suite'. Use one of: $(suites)"
# A suite that runs with several settings has a variant input per job.
# VARIANT picks one of them; the first one is the default.
job="$(jq -c --arg variant "${VARIANT:-}" 'select($variant == "" or .with.variant == $variant)' <<< "$candidates" | head -n 1)"
[ -n "$job" ] || fail "CI doesn't run '$suite' with variant '${VARIANT:-}'. Use one of: $(jq -r '.with.variant // empty' <<< "$candidates" | tr '\n' ' ')"

defaults="$(yq -o=json '.inputs' "$action" | jq -c 'map_values(.default // "")')"
nodes="$(yq '.jobs.matrix.steps[] | select(.id == "matrix") | .env.NODES' "$matrix")"
kind_version="$(yq '.jobs.matrix.steps[] | select(.id == "matrix") | .env.KIND_VERSION' "$matrix")"

# The versions the job tests: the matrix output that its k8s-version uses,
# computed with the same jq filter as kubernetes-matrix.yaml. A job that
# picks between versions and smoke-versions, such as the certificate jobs,
# uses smoke-versions outside release-candidate runs.
expression="$(jq -r '.versions // ""' <<< "$job")"
output=versions
for name in smoke-versions beta-versions stable-versions; do
  if [[ "$expression" == *"outputs.$name"* ]]; then
    output="$name"
    break
  fi
done
script="$(yq '.jobs.matrix.steps[] | select(.id == "matrix") | .run' "$matrix")"
filter="$(sed -n "s/.*echo \"$output=\$(jq -c '\(.*\)' <<< \"\$NODES\")\".*/\1/p" <<< "$script")"
[ -n "$filter" ] || fail "can't find how $matrix computes $output"
tested="$(jq -c --argjson versions "$(jq -c "$filter" <<< "$nodes")" \
  'map(select(.version as $v | $versions | index($v)))' <<< "$nodes")"

if [ -z "$k8s" ]; then
  node="$(jq -c 'last' <<< "$tested")"
else
  want="v${k8s#v}"
  node="$(jq -c --arg v "$want" 'map(select(.version == $v or (.version | startswith($v + ".")))) | last // empty' <<< "$nodes")"
  [ -n "$node" ] || fail "no node image for Kubernetes $k8s. Use one of: $(jq -r 'map(.version) | join(" ")' <<< "$nodes")"
  if ! jq -e --argjson n "$node" 'any(.[]; .version == $n.version)' <<< "$tested" > /dev/null; then
    echo "settings.sh: CI doesn't run $suite on $(jq -r '.version' <<< "$node"); running it anyway." >&2
  fi
fi

settings="$(jq -n --argjson job "$job" --argjson defaults "$defaults" --argjson node "$node" --arg kind "$kind_version" '
  ($defaults + $job.with) as $w | {
    "k8s-version": $node.version,
    "kind-image": $node.image,
    "kind-version": $kind,
    "kind-config": ($w["kind-config"] | sub(".*/"; "") | sub("\\.yaml$"; "")),
    "kyverno-configs": $w["kyverno-configs"],
    "explicit-install-settings": $w["explicit-install-settings"],
    "install-cert-manager": $w["install-cert-manager"],
    "install-openreports": $w["install-openreports"],
    "install-kubectl-evict": $w["install-kubectl-evict"],
    "variant": $w["variant"],
    "job": $job.job
  }')"

if [ -n "$key" ]; then
  jq -er --arg key "$key" '.[$key] // error("unknown key \($key)")' <<< "$settings"
else
  jq -r 'to_entries[] | "\(.key)=\(.value)"' <<< "$settings"
fi
