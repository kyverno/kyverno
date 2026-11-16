#!/usr/bin/env bash
# Tests test-e2e-local.sh. It runs the script in a small copy of the repository
# layout, with stubs for kind, kubectl, docker, helm, make, go and chainsaw, so
# it needs no cluster. It also checks that the script can still read the real
# CI files.
#
# Needs bash and yq v4. Set YQ to the path of yq when it isn't on your PATH, or
# run `make test-e2e-local-unit`, which installs it.
#
# Usage: scripts/test-e2e-local_test.sh
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
if ! YQ=$(command -v "${YQ:-yq}"); then
  echo "yq not found: set YQ, or run make test-e2e-local-unit."
  exit 1
fi
unset SUITE TEST K8S USE_CONFIG RUN_QUARANTINED DRY_RUN E2E_KIND_NAME
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
repo=$work/repo
bin=$work/bin
state=$work/tools/e2e-local/kyverno-e2e

failures=0
fail() {
  echo "FAIL: $name: $*"
  failures=$((failures + 1))
}

# The stub tools. They log each call in $work/calls. kind keeps its clusters in
# $work/clusters, chainsaw writes a report, and make "installs" yq.
mkdir -p "$bin" "$work/tools"
cat > "$bin/stub" << 'EOF'
#!/usr/bin/env bash
tool=$(basename "$0")
echo "$tool $* [MAKEFLAGS=${MAKEFLAGS:-}]" >> "$STUB_DIR/calls"
arg() {
  local want=$1
  shift
  while [ $# -gt 1 ]; do
    [ "$1" != "$want" ] || echo "$2"
    shift
  done
}
case "$tool ${1:-} ${2:-}" in
  "kind version "*) echo "kind v0.33.0 go1.27.3 linux/amd64" ;;
  "kind get clusters") cat "$STUB_DIR/clusters" ;;
  "kind create cluster")
    arg --name "$@" >> "$STUB_DIR/clusters"
    exit "${STUB_KIND_CREATE:-0}"
    ;;
  "kind delete cluster")
    grep -v -x "$(arg --name "$@")" "$STUB_DIR/clusters" > "$STUB_DIR/clusters.new" || true
    mv "$STUB_DIR/clusters.new" "$STUB_DIR/clusters"
    ;;
  "docker info "*) exit "${STUB_DOCKER:-0}" ;;
  "chainsaw version "*) echo "Version: 0.2.15-beta.3" ;;
  "chainsaw test "*)
    report='{"name":"chainsaw-report","tests":[{"name":"a"}]}'
    echo "${STUB_REPORT:-$report}" > "$(arg --report-path "$@")/results.json"
    exit "${STUB_CHAINSAW:-0}"
    ;;
  "make "*)
    for last in "$@"; do :; done
    case "$last" in */yq) ln -s "$STUB_YQ" "$last" ;; esac
    ;;
esac
exit 0
EOF
chmod +x "$bin/stub"
for tool in kind kubectl docker helm make go chainsaw; do ln -s stub "$bin/$tool"; done
: > "$work/clusters"

# The copy of the repository layout: the script, the CI files it reads, and the
# directories it checks.
mkdir -p "$repo/scripts/config/kind" "$repo/.github/workflows" \
  "$repo/.github/actions/tests/conformance/run" "$repo/.github/actions/tools/chainsaw" \
  "$repo/test/conformance/chainsaw/generating-policies/clone/sync"
cp "$here/test-e2e-local.sh" "$repo/scripts/"
for suite in ttl rbac generating-policies mutating-admission-policy-reports cli \
  tls-certificates/rsa-cert-manager tls-certificates/ecdsa-cert-manager; do
  mkdir -p "$repo/test/conformance/chainsaw/$suite"
  : > "$repo/test/conformance/chainsaw/$suite/.chainsaw.yaml"
done
for config in standard default ttl force-failure-policy-ignore mutating-admission-policy-reports; do
  mkdir -p "$repo/scripts/config/$config"
  : > "$repo/scripts/config/$config/kyverno.yaml"
done
for config in default vap-v1beta1; do
  cat > "$repo/scripts/config/kind/$config.yaml" << 'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
  - role: worker
EOF
done
cat > "$repo/test/conformance/quarantine.yaml" << 'EOF'
tests:
  - path: generating-policies/clone/sync/sync-modify-downstream
    owner: me
  - path: generating-policies-v1/other
    owner: me
EOF
cat > "$repo/.github/actions/tests/conformance/run/action.yaml" << 'EOF'
inputs:
  k8s-version:
    required: true
  kind-config:
    default: ./scripts/config/kind/default.yaml
  kyverno-configs:
    default: standard
  tests-path:
    default: '.'
  install-cert-manager:
    default: 'false'
  install-kubectl-evict:
    default: 'false'
  install-openreports:
    default: 'false'
  explicit-install-settings:
    default: ''
EOF
cat > "$repo/.github/actions/tools/chainsaw/action.yaml" << 'EOF'
runs:
  steps:
    - uses: kyverno/action-install-chainsaw@v0.2.14
      with:
        verify: true
        release: v0.2.15-beta.3
EOF
cat > "$repo/.github/workflows/kubernetes-matrix.yaml" << 'EOF'
jobs:
  matrix:
    steps:
      - name: Export matrix
        id: matrix
        env:
          KIND_VERSION: v0.33.0
          NODES: |
            [
              {"version": "v1.34.11", "image": "kindest/node:v1.34.11@sha256:aaa", "mutatingAdmissionPolicy": "v1beta1"},
              {"version": "v1.35.8", "image": "kindest/node:v1.35.8@sha256:bbb", "mutatingAdmissionPolicy": "v1beta1"},
              {"version": "v1.36.4", "image": "kindest/node:v1.36.4@sha256:ccc", "mutatingAdmissionPolicy": "v1"}
            ]
EOF
cat > "$repo/.github/workflows/tests-conformance.yaml" << 'EOF'
jobs:
  kubernetes-matrix:
    uses: ./.github/workflows/kubernetes-matrix.yaml
  ttl:
    strategy:
      matrix:
        k8s-version: ${{ fromJSON(needs.kubernetes-matrix.outputs.versions) }}
    steps:
      - uses: actions/checkout@v7
      - uses: ./.github/actions/tests/conformance/run
        with:
          k8s-version: ${{ matrix.k8s-version }}
          kyverno-configs: standard,ttl
          tests-path: ttl
  rbac:
    strategy:
      matrix:
        kyverno-configs: [standard, default, force-failure-policy-ignore]
        k8s-version: ${{ fromJSON(needs.kubernetes-matrix.outputs.versions) }}
    steps:
      - uses: ./.github/actions/tests/conformance/run
        with:
          k8s-version: ${{ matrix.k8s-version }}
          kyverno-configs: ${{ matrix.kyverno-configs }}
          tests-path: rbac
  generating-policies:
    strategy:
      matrix:
        k8s-version: ${{ fromJSON(needs.kubernetes-matrix.outputs.versions) }}
        shard-index: [0, 1]
    steps:
      - uses: ./.github/actions/tests/conformance/run
        with:
          k8s-version: ${{ matrix.k8s-version }}
          tests-path: generating-policies
          install-kubectl-evict: true
          shard-index: ${{ matrix.shard-index }}
          shard-count: 2
  mutating-admission-policy-reports:
    strategy:
      matrix:
        k8s-version: ${{ fromJSON(needs.kubernetes-matrix.outputs.beta-versions) }}
    steps:
      - uses: ./.github/actions/tests/conformance/run
        with:
          k8s-version: ${{ matrix.k8s-version }}
          kind-config: ./scripts/config/kind/vap-v1beta1.yaml
          kyverno-configs: standard,mutating-admission-policy-reports
          tests-path: mutating-admission-policy-reports
  cert-manager-certificates:
    strategy:
      matrix:
        k8s-version: ${{ fromJSON(needs.kubernetes-matrix.outputs.smoke-versions) }}
        key-algorithm:
          - name: rsa
            test-dir: rsa-cert-manager
            helm-settings: --set a=RSA --set b=RSA
          - name: ecdsa
            test-dir: ecdsa-cert-manager
            helm-settings: --set a=ECDSA
    steps:
      - uses: ./.github/actions/tests/conformance/run
        with:
          k8s-version: ${{ matrix.k8s-version }}
          kyverno-configs: standard
          tests-path: tls-certificates/${{ matrix.key-algorithm.test-dir }}
          install-cert-manager: 'true'
          explicit-install-settings: ${{ matrix.key-algorithm.helm-settings }}
  helm-tests:
    steps:
      - run: make helm-test
EOF

export YQ STUB_YQ="$YQ" STUB_DIR="$work" TOOLS_DIR="$work/tools" PATH="$bin:$PATH"
export KIND="$bin/kind" HELM="$bin/helm" CHAINSAW="$bin/chainsaw" MAKE="$bin/make"

# Runs the script with the given environment and arguments; sets out and rc.
e2e() {
  : > "$work/calls"
  rc=0
  out=$(env "$@" 2>&1) || rc=$?
}
run() { e2e "$@" "$repo/scripts/test-e2e-local.sh"; }
dry() { run DRY_RUN=1 "$@"; }
clean() { e2e "$@" "$repo/scripts/test-e2e-local.sh" clean; }

expect_rc() { [ "$rc" = "$1" ] || fail "exit code $rc, want $1. Output: $out"; }
expect_out() { grep -qF -- "$1" <<< "$out" || fail "the output lacks '$1'. Output: $out"; }
expect_no_out() { ! grep -qF -- "$1" <<< "$out" || fail "the output has '$1'. Output: $out"; }
expect_call() { grep -qF -- "$1" "$work/calls" || fail "no call of '$1'. Calls: $(cat "$work/calls")"; }
expect_no_call() { ! grep -qF -- "$1" "$work/calls" || fail "a call of '$1'. Calls: $(cat "$work/calls")"; }

name="setup of a job"
dry SUITE=ttl
expect_rc 0
expect_out "Suite:       ttl (CI job: ttl)"
expect_out "Kubernetes:  v1.36.4 (kindest/node:v1.36.4@sha256:ccc)"
expect_out "kind-install-kyverno USE_CONFIG=standard,ttl EXPLICIT_INSTALL_SETTINGS="
expect_out "--include-test-regex '^chainsaw\$/'"
expect_no_out "--exclude-test-regex"
expect_no_call "kind create"

name="suite given as a path"
dry SUITE=./test/conformance/chainsaw/ttl/
expect_rc 0
expect_out "Suite:       ttl (CI job: ttl)"

name="defaults of the action"
dry SUITE=generating-policies TEST=clone/sync
expect_rc 0
expect_out "config scripts/config/kind/default.yaml"
expect_out "kind-install-kyverno USE_CONFIG=standard EXPLICIT_INSTALL_SETTINGS="
expect_out "go install github.com/ueokande/kubectl-evict@latest"
expect_out "--include-test-regex '^chainsaw\$/clone/sync'"
expect_no_out "CI runs this suite with"

name="one setup for each matrix value"
dry SUITE=rbac
expect_rc 0
expect_out "CI runs this suite with 3 setups"
expect_out "kind-install-kyverno USE_CONFIG=standard EXPLICIT_INSTALL_SETTINGS="
dry SUITE=rbac USE_CONFIG=force-failure-policy-ignore
expect_out "kind-install-kyverno USE_CONFIG=force-failure-policy-ignore EXPLICIT_INSTALL_SETTINGS="
dry SUITE=rbac USE_CONFIG=standard,nope
expect_rc 1
expect_out "Unknown Kyverno config nope"

name="path and settings from a matrix value"
dry SUITE=tls-certificates/ecdsa-cert-manager
expect_rc 0
expect_out "Suite:       tls-certificates/ecdsa-cert-manager (CI job: cert-manager-certificates)"
expect_out "helm upgrade --install cert-manager cert-manager"
expect_out "kind-install-kyverno USE_CONFIG=standard 'EXPLICIT_INSTALL_SETTINGS=--set a=ECDSA'"
dry SUITE=tls-certificates/rsa-cert-manager
expect_out "'EXPLICIT_INSTALL_SETTINGS=--set a=RSA --set b=RSA'"

name="suite that runs on the beta versions"
dry SUITE=mutating-admission-policy-reports
expect_rc 0
expect_out "Kubernetes:  v1.35.8 (kindest/node:v1.35.8@sha256:bbb)"
expect_out "config scripts/config/kind/vap-v1beta1.yaml"
expect_out "USE_CONFIG=standard,mutating-admission-policy-reports"
expect_no_out "Warning"
dry SUITE=mutating-admission-policy-reports K8S=1.36
expect_rc 0
expect_out "Warning: CI doesn't run mutating-admission-policy-reports on Kubernetes v1.36.4, only on v1.34.11, v1.35.8."

name="Kubernetes version"
dry SUITE=ttl K8S=1.34
expect_out "Kubernetes:  v1.34.11 (kindest/node:v1.34.11@sha256:aaa)"
dry SUITE=ttl K8S=v1.35.8
expect_out "--image kindest/node:v1.35.8@sha256:bbb"
dry SUITE=ttl K8S=1.3
expect_rc 1
expect_out "Kubernetes 1.3 is not in the CI matrix"
expect_out "Supported versions: v1.34.11, v1.35.8, v1.36.4"

name="quarantined tests"
dry SUITE=generating-policies
expect_out "Quarantined: skipping clone/sync/sync-modify-downstream (as CI does"
expect_out "--exclude-test-regex '^chainsaw\$/^clone\$/^sync\$/^sync-modify-downstream\\['"
dry SUITE=generating-policies RUN_QUARANTINED=true
expect_rc 0
expect_no_out "--exclude-test-regex"

name="wrong suite"
dry
expect_rc 1
expect_out "SUITE is not set"
dry SUITE=nope
expect_rc 1
expect_out "nope is not a suite"
dry SUITE=generating-policies/clone/sync
expect_rc 1
expect_out "make test-e2e-local SUITE=generating-policies TEST=clone/sync"
dry SUITE=tls-certificates
expect_rc 1
expect_out "holds more than one suite"
expect_out "  tls-certificates/rsa-cert-manager"

name="suite without a job"
dry SUITE=cli
expect_rc 1
expect_out "No job in .github/workflows/tests-conformance.yaml runs the suite cli"
dry SUITE=cli USE_CONFIG=default
expect_rc 0
expect_out "kind-install-kyverno USE_CONFIG=default EXPLICIT_INSTALL_SETTINGS="

name="dry run without yq"
dry SUITE=ttl YQ="$work/tools/yq" MAKEFLAGS="n -- SUITE=ttl" MFLAGS=-n
expect_rc 0
expect_out "Installing yq at $work/tools/yq"
expect_call "make YQ=$work/tools/yq TOOLS_DIR=$work/tools $work/tools/yq [MAKEFLAGS=]"
expect_out "USE_CONFIG=standard,ttl"

name="Docker isn't running"
run SUITE=ttl STUB_DOCKER=1
expect_rc 1
expect_out "Docker isn't running"
expect_no_call "kind create"

name="new cluster"
run SUITE=ttl
expect_rc 0
expect_call "kind create cluster --name kyverno-e2e --image kindest/node:v1.36.4@sha256:ccc --config $state/kind.yaml --kubeconfig $state/kubeconfig"
expect_call "make kind-load-all KIND_NAME=kyverno-e2e KIND=$bin/kind"
expect_call "make kind-install-kyverno USE_CONFIG=standard,ttl"
expect_call "kubectl wait --namespace kyverno"
expect_call "chainsaw test --config .chainsaw.yaml"
expect_no_call "kind delete"
expect_out "To delete it: make test-e2e-local-clean"
grep -q "role: worker" "$state/kind.yaml" || fail "the kind config lacks the nodes"
! grep -q extraPortMappings "$state/kind.yaml" || fail "the kind config maps host ports"

name="same setup"
run SUITE=ttl TEST=other
expect_rc 0
expect_out "Reusing the kind cluster kyverno-e2e"
expect_no_call "kind create"
expect_no_call "kind delete"
expect_call "kubectl --namespace kyverno rollout restart deployment"

name="another Kubernetes version"
run SUITE=ttl K8S=1.35
expect_rc 0
expect_out "has another setup: recreating it"
expect_call "kind delete cluster --name kyverno-e2e"
expect_call "kind create cluster --name kyverno-e2e --image kindest/node:v1.35.8@sha256:bbb"

name="another Kyverno setup"
run SUITE=rbac K8S=1.35
expect_call "kind delete cluster --name kyverno-e2e"
expect_call "kind create cluster"

name="changed kind config"
printf 'featureGates:\n  SomeGate: true\n' >> "$repo/scripts/config/kind/default.yaml"
run SUITE=rbac K8S=1.35
expect_rc 0
expect_call "kind delete cluster --name kyverno-e2e"
expect_call "kind create cluster"
grep -q SomeGate "$state/kind.yaml" || fail "the kind config lacks the change"
run SUITE=rbac K8S=1.35
expect_out "Reusing the kind cluster kyverno-e2e"

name="failing tests"
run SUITE=rbac K8S=1.35 STUB_CHAINSAW=3
expect_rc 3

name="no test ran"
run SUITE=rbac K8S=1.35 TEST=nothing STUB_REPORT='{"name":"chainsaw-report"}'
expect_rc 1
expect_out "Error: no test ran. TEST=nothing matches no test of rbac"

name="cluster of someone else"
echo other >> "$work/clusters"
run SUITE=ttl E2E_KIND_NAME=other
expect_rc 1
expect_out "A kind cluster named other exists, and test-e2e-local didn't create it"
expect_out "E2E_KIND_NAME=<name>"
expect_no_call "kind delete"
expect_no_call "kind create"
dry SUITE=ttl E2E_KIND_NAME=other
expect_rc 1
clean E2E_KIND_NAME=other
expect_rc 0
expect_out "nothing to delete"
expect_no_call "kind delete"
grep -q -x other "$work/clusters" || fail "the cluster is gone"

name="failed creation"
run SUITE=ttl E2E_KIND_NAME=broken STUB_KIND_CREATE=1
expect_rc 1
run SUITE=ttl E2E_KIND_NAME=broken
expect_rc 0
expect_call "kind delete cluster --name broken"
expect_call "kind create cluster --name broken"

name="clean"
clean
expect_rc 0
expect_call "kind delete cluster --name kyverno-e2e"
[ ! -e "$state" ] || fail "$state is still there"
! grep -q -x kyverno-e2e "$work/clusters" || fail "the cluster is still there"
clean
expect_rc 0
expect_out "nothing to delete"
expect_no_call "kind delete"

# The script must still understand the real CI files: every suite either gets
# a setup, or isn't run by a job of the conformance workflow.
name="CI files of this repository"
real=$(cd "$here/.." && pwd)
with_setup=0
without_job=0
for suite in $(cd "$real/test/conformance/chainsaw" && find . -name .chainsaw.yaml | sed -e 's|^\./||' -e 's|/\.chainsaw\.yaml$||' | sort); do
  e2e DRY_RUN=1 SUITE="$suite" "$real/scripts/test-e2e-local.sh"
  if [ "$rc" = 0 ]; then
    with_setup=$((with_setup + 1))
    expect_out "kind-install-kyverno USE_CONFIG="
  else
    without_job=$((without_job + 1))
    expect_out "No job in .github/workflows/tests-conformance.yaml runs the suite $suite"
  fi
done
[ "$with_setup" -gt "$without_job" ] || fail "$with_setup suites have a setup, $without_job have no job"

if [ "$failures" -gt 0 ]; then
  echo "$failures failed"
  exit 1
fi
echo "ok"
