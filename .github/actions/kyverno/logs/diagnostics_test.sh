#!/usr/bin/env bash
# Tests diagnostics.sh against a fake kubectl, so it needs no cluster.
#
# Usage: .github/actions/kyverno/logs/diagnostics_test.sh
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
script=$here/diagnostics.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

failures=0
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

# The fake kubectl logs its arguments to $FAKE_CALLS. Calls whose arguments
# match the regex $FAKE_FAIL exit 1, calls that match $FAKE_HANG never return.
mkdir "$work/bin"
cat > "$work/bin/kubectl" << 'EOF'
#!/usr/bin/env bash
echo "$*" >> "$FAKE_CALLS"
if [ -n "${FAKE_HANG:-}" ] && [[ "$*" =~ $FAKE_HANG ]]; then exec sleep 300; fi
if [ -n "${FAKE_FAIL:-}" ] && [[ "$*" =~ $FAKE_FAIL ]]; then
  echo "error: fake failure for $1" >&2
  echo "second line" >&2
  exit 1
fi
case "$*" in
  "api-resources -o name")
    printf '%s\n' pods secrets deployments.apps certificates.cert-manager.io \
      clusterpolicies.kyverno.io policies.kyverno.io policyexceptions.kyverno.io updaterequests.kyverno.io \
      validatingpolicies.policies.kyverno.io policyexceptions.policies.kyverno.io \
      ephemeralreports.reports.kyverno.io policyreports.wgpolicyk8s.io reports.openreports.io \
      mutatingwebhookconfigurations.admissionregistration.k8s.io \
      validatingwebhookconfigurations.admissionregistration.k8s.io \
      validatingadmissionpolicies.admissionregistration.k8s.io
    ;;
  *go-template=*)
    echo "admission-abc kyverno-pre 0"
    echo "admission-abc kyverno 2"
    echo "reports-def controller 0"
    ;;
  *) echo "output of: $*" ;;
esac
EOF
chmod +x "$work/bin/kubectl"

# Runs diagnostics.sh; sets out, rc, bundle, readme, and calls.
# Arguments are environment variables for the script and the fake kubectl.
run() {
  bundle=$work/bundle
  rm -rf "$bundle"
  : > "$work/calls"
  rc=0
  out=$(env PATH="$work/bin:$PATH" FAKE_CALLS="$work/calls" "$@" "$script" "$bundle" 2>&1) || rc=$?
  readme=$(cat "$bundle/README.md" 2> /dev/null || true)
  calls=$(cat "$work/calls")
}

expect_rc() { [ "$rc" = "$1" ] || fail "$name: exit code $rc, want $1. Output: $out"; }
expect_file() { grep -qF -- "$2" "$bundle/$1" 2> /dev/null || fail "$name: $1 lacks '$2'"; }
expect_no_file() { [ ! -e "$bundle/$1" ] || fail "$name: unexpected $1"; }
expect_readme() { grep -qF -- "$1" <<< "$readme" || fail "$name: README.md lacks '$1'. README.md: $readme"; }
expect_called() { grep -qE -- "$1" <<< "$calls" || fail "$name: kubectl wasn't called with '$1'"; }
expect_not_called() { ! grep -qE -- "$1" <<< "$calls" || fail "$name: kubectl was called with '$1'"; }

name="healthy cluster"
run
expect_rc 0
expect_file cluster/version.txt "output of: version"
expect_file cluster/nodes.txt "get nodes"
expect_file cluster/pods.txt "get pods -A"
expect_file cluster/events.txt "get events -A --sort-by=.metadata.creationTimestamp"
expect_file cluster/api-resources.txt "clusterpolicies.kyverno.io"
expect_file webhooks/validatingwebhookconfigurations.yaml "get validatingwebhookconfigurations -o yaml"
expect_file webhooks/mutatingwebhookconfigurations.yaml "get mutatingwebhookconfigurations -o yaml"
expect_file kyverno/pods.txt "-n kyverno get pods -o wide"
expect_file kyverno/describe-pods.txt "-n kyverno describe pods"
expect_file kyverno/containers.txt "admission-abc kyverno 2"
expect_file kyverno/logs/admission-abc/kyverno-pre.log "logs admission-abc -c kyverno-pre"
expect_file kyverno/logs/admission-abc/kyverno.log "logs admission-abc -c kyverno"
expect_file kyverno/logs/admission-abc/kyverno.previous.log "logs admission-abc -c kyverno --previous"
expect_file kyverno/logs/reports-def/controller.log "logs reports-def -c controller"
# Only a container that restarted has previous logs.
expect_no_file kyverno/logs/admission-abc/kyverno-pre.previous.log
expect_no_file kyverno/logs/reports-def/controller.previous.log
for resource in clusterpolicies.kyverno.io policies.kyverno.io policyexceptions.kyverno.io \
  validatingpolicies.policies.kyverno.io policyexceptions.policies.kyverno.io \
  validatingadmissionpolicies.admissionregistration.k8s.io; do
  expect_file "policies/$resource.yaml" "get $resource -A -o yaml"
done
for resource in ephemeralreports.reports.kyverno.io policyreports.wgpolicyk8s.io reports.openreports.io; do
  expect_file "reports/$resource.yaml" "get $resource -A -o yaml"
done
# UpdateRequests carry the admission request, so only chosen columns are read.
expect_file policies/updaterequests.kyverno.io.txt "custom-columns="
expect_not_called 'updaterequests.*-o yaml'
expect_not_called 'spec\.context'
# Nothing outside the Kyverno, report and admission groups is read.
expect_not_called 'get (secrets|pods|deployments\.apps|certificates\.cert-manager\.io) -A -o yaml'
expect_not_called '[Ss]ecret'
[ "$(grep -c 'get validatingwebhookconfigurations' <<< "$calls")" = 1 ] || fail "$name: webhook configurations read twice"
expect_readme "- cluster/events.txt: kubectl get events"
expect_readme "- kyverno/logs/admission-abc/kyverno.previous.log: kubectl -n kyverno logs admission-abc -c kyverno --previous"
expect_readme "Nothing failed."
grep -qE '^Collected [0-9]+ files into .*, 0 commands failed\.$' <<< "$out" || fail "$name: unexpected output: $out"
[ "$(wc -l <<< "$out" | tr -d ' ')" = 1 ] || fail "$name: more than one line of output: $out"

name="another namespace"
run KYVERNO_NAMESPACE=other
expect_file kyverno/logs/reports-def/controller.log "-n other logs reports-def -c controller"

name="failing commands"
run FAKE_FAIL='^get nodes|policyexceptions\.kyverno\.io|logs admission-abc -c kyverno$|describe'
expect_rc 0
expect_no_file cluster/nodes.txt
expect_no_file kyverno/describe-pods.txt
expect_no_file kyverno/logs/admission-abc/kyverno.log
expect_no_file policies/policyexceptions.kyverno.io.yaml
expect_readme "- cluster/nodes.txt: kubectl get nodes -o wide: exit code 1: error: fake failure for get"
expect_readme "- policies/policyexceptions.kyverno.io.yaml: kubectl get policyexceptions.kyverno.io -A -o yaml: exit code 1"
expect_readme "- kyverno/logs/admission-abc/kyverno.log: kubectl -n kyverno logs admission-abc -c kyverno: exit code 1"
! grep -qF "second line" <<< "$readme" || fail "$name: README.md quotes more than the first error line"
! grep -qF "Nothing failed." <<< "$readme" || fail "$name: README.md says nothing failed"
# What comes after a failure is still collected.
expect_file cluster/pods.txt "get pods -A"
expect_file kyverno/logs/admission-abc/kyverno.previous.log "--previous"
expect_file kyverno/logs/reports-def/controller.log "logs reports-def"
expect_file policies/policyexceptions.policies.kyverno.io.yaml "get policyexceptions.policies.kyverno.io"
expect_file reports/reports.openreports.io.yaml "get reports.openreports.io"
grep -qE ', 4 commands failed\.$' <<< "$out" || fail "$name: unexpected output: $out"

name="hanging command"
start=$SECONDS
run DIAGNOSTICS_TIMEOUT=1 FAKE_HANG='^get events'
expect_rc 0
[ $((SECONDS - start)) -lt 30 ] || fail "$name: took $((SECONDS - start))s"
expect_no_file cluster/events.txt
expect_readme "- cluster/events.txt: kubectl get events"
expect_readme "timed out after 1s"
expect_file webhooks/validatingwebhookconfigurations.yaml "get validatingwebhookconfigurations"
expect_file reports/policyreports.wgpolicyk8s.io.yaml "get policyreports.wgpolicyk8s.io"

name="unreachable cluster"
run FAKE_FAIL='.'
expect_rc 0
expect_readme "- cluster/version.txt: kubectl version: exit code 1"
expect_readme "- cluster/api-resources.txt: kubectl api-resources -o name: exit code 1"
expect_no_file policies
expect_no_file reports
expect_no_file kyverno/logs
[ "$(find "$bundle" -type f | wc -l | tr -d ' ')" = 1 ] || fail "$name: files other than README.md: $(find "$bundle" -type f)"

name="kinds that aren't installed"
cat > "$work/bin/kubectl-bare" << 'EOF'
#!/usr/bin/env bash
case "$*" in
  "api-resources -o name") printf '%s\n' pods secrets validatingwebhookconfigurations.admissionregistration.k8s.io ;;
  *go-template=*) ;;
  *) echo "output of: $*" ;;
esac
EOF
chmod +x "$work/bin/kubectl-bare"
mkdir "$work/bare"
ln -s "$work/bin/kubectl-bare" "$work/bare/kubectl"
run PATH="$work/bare:$PATH"
expect_rc 0
expect_file cluster/events.txt "get events"
expect_no_file policies
expect_no_file reports
expect_no_file kyverno/logs
expect_readme "Nothing failed."

name="time budget"
run DIAGNOSTICS_BUDGET=0
expect_rc 0
[ -z "$calls" ] || fail "$name: kubectl was called: $calls"
expect_readme "- cluster/version.txt: kubectl version: skipped, the 0s budget was used up"

name="output directory that can't be created"
: > "$work/file"
rc=0
out=$(PATH="$work/bin:$PATH" FAKE_CALLS="$work/calls" "$script" "$work/file/bundle" 2>&1) || rc=$?
[ "$rc" != 0 ] || fail "$name: exit code 0"

if [ "$failures" -gt 0 ]; then
  echo "$failures check(s) failed."
  exit 1
fi
echo "All diagnostics.sh checks passed."
