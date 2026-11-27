# Shared assertions for the AuthorizingPolicy e2e tests. Source it from a
# chainsaw script (the working directory is the test directory):
#   . ../_lib/authz.sh
#
# Each assertion runs a real kubectl request, usually impersonating a test user
# with --as, and checks the decision kube-apiserver enforced, not only the
# HTTP status of a Kyverno endpoint.

# expect_allowed DESCRIPTION COMMAND...
# Retries for up to 60s, because Kyverno reconciles policies asynchronously.
expect_allowed() {
  desc=$1
  shift
  last=""
  for _ in $(seq 1 30); do
    if last=$("$@" 2>&1); then
      echo "ALLOWED (expected): ${desc}"
      return 0
    fi
    sleep 2
  done
  echo "FAIL: expected ALLOWED: ${desc}" >&2
  echo "  last output: ${last}" >&2
  return 1
}

# expect_forbidden DESCRIPTION COMMAND...
# The request must fail with an authorization Forbidden error ("is forbidden:
# User ... cannot ..."), not a validation error. No retry: call it
# after an expect_allowed that proves the policy is active.
expect_forbidden() {
  desc=$1
  shift
  if out=$("$@" 2>&1); then
    echo "FAIL: expected FORBIDDEN but the request succeeded: ${desc}" >&2
    echo "  output: ${out}" >&2
    return 1
  fi
  if ! echo "${out}" | grep -q 'is forbidden: User '; then
    echo "FAIL: expected FORBIDDEN but got a different error: ${desc}" >&2
    echo "  output: ${out}" >&2
    return 1
  fi
  echo "FORBIDDEN (expected): ${desc}"
  echo "  ${out}"
}

# expect_eventually_forbidden DESCRIPTION COMMAND...
# Like expect_forbidden, but retries for up to 60s (for policy updates or deletions).
expect_eventually_forbidden() {
  desc=$1
  shift
  out=""
  for _ in $(seq 1 30); do
    if ! out=$("$@" 2>&1) && echo "${out}" | grep -q 'is forbidden: User '; then
      echo "FORBIDDEN (expected): ${desc}"
      echo "  ${out}"
      return 0
    fi
    sleep 2
  done
  echo "FAIL: expected FORBIDDEN: ${desc}" >&2
  echo "  last output: ${out}" >&2
  return 1
}

# expect_rejected DESCRIPTION PATTERN COMMAND...
# The request must fail and its output must match the extended regex PATTERN.
expect_rejected() {
  desc=$1
  pattern=$2
  shift 2
  if out=$("$@" 2>&1); then
    echo "FAIL: expected REJECTED but the request succeeded: ${desc}" >&2
    echo "  output: ${out}" >&2
    return 1
  fi
  if ! echo "${out}" | grep -q -E "${pattern}"; then
    echo "FAIL: rejected, but the output does not match /${pattern}/: ${desc}" >&2
    echo "  output: ${out}" >&2
    return 1
  fi
  echo "REJECTED (expected): ${desc}"
  echo "  ${out}"
}

# tofile: saves stdin to a new temporary file and prints its path, so that
# retried commands can read the same manifest more than once.
tofile() {
  f=$(mktemp)
  cat > "${f}"
  echo "${f}"
}

# configmap NAME [LABEL=VALUE...]: prints a ConfigMap manifest.
configmap() {
  name=$1
  shift
  echo "apiVersion: v1"
  echo "kind: ConfigMap"
  echo "metadata:"
  echo "  name: ${name}"
  if [ $# -gt 0 ]; then
    echo "  labels:"
    for kv in "$@"; do
      echo "    ${kv%%=*}: \"${kv#*=}\""
    done
  fi
  echo "data:"
  echo "  key: value"
}

# sar USER VERB RESOURCE NAMESPACE [GROUP...]: creates a SubjectAccessReview
# through the API (an unconditional caller that does not set
# handledDecisionTypes) and prints "allowed=<bool> denied=<bool>"; an omitted
# field prints as false.
sar() {
  user=$1 verb=$2 resource=$3 ns=$4
  shift 4
  groups=""
  for g in "$@"; do groups="${groups}\"${g}\","; done
  status=$(kubectl create -o jsonpath='{.status.allowed} {.status.denied}' -f - <<EOF
{"apiVersion":"authorization.k8s.io/v1","kind":"SubjectAccessReview",
 "spec":{"user":"${user}","groups":[${groups%,}],
  "resourceAttributes":{"verb":"${verb}","resource":"${resource}","namespace":"${ns}","version":"v1"}}}
EOF
)
  allowed=${status%% *}
  denied=${status#* }
  echo "allowed=${allowed:-false} denied=${denied:-false}"
}

# expect_absent KIND NAME...: none of the named objects exist in $NAMESPACE.
expect_absent() {
  kind=$1
  shift
  for n in "$@"; do
    if kubectl -n "${NAMESPACE}" get "${kind}" "${n}" >/dev/null 2>&1; then
      echo "FAIL: ${kind}/${n} exists but its creation should have been rejected" >&2
      return 1
    fi
  done
  echo "ABSENT (expected): ${kind} $*"
}
