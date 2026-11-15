#!/usr/bin/env bash
# The expected summaries contain literal Markdown backticks.
# shellcheck disable=SC2016
#
# Tests quarantine.sh. Needs bash, yq v4, and GNU or BSD date.
#
# Usage: .github/actions/tests/conformance/run/quarantine_test.sh
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../../../../.." && pwd)
script=$here/quarantine.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

failures=0
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

# Creates a fresh chainsaw tree with three tests.
setup() {
  rm -rf "$work/chainsaw"
  for t in suite/a suite-v1/a other/b; do
    mkdir -p "$work/chainsaw/$t"
    printf 'apiVersion: chainsaw.kyverno.io/v1alpha1\nkind: Test\nspec:\n  steps: []\n' > "$work/chainsaw/$t/chainsaw-test.yaml"
  done
}

# Runs quarantine.sh; sets out, rc, and summary.
run() {
  local file=$1 suite=$2 today=${3:-2026-06-01}
  : > "$work/summary"
  rc=0
  out=$(GITHUB_STEP_SUMMARY=$work/summary "$script" "$file" "$work/chainsaw" "$suite" "$today" 2>&1) || rc=$?
  summary=$(cat "$work/summary")
}

skipped() { [ "$(yq '.spec.skip' "$work/chainsaw/$1/chainsaw-test.yaml")" = true ]; }
expect_skipped() { skipped "$1" || fail "$name: $1 isn't skipped"; }
expect_runs() { ! skipped "$1" || fail "$name: $1 is skipped"; }
expect_rc() { [ "$rc" = "$1" ] || fail "$name: exit code $rc, want $1. Output: $out"; }
expect_reported() { grep -qF -- "$1" <<< "$summary" || fail "$name: summary lacks '$1'. Summary: $summary"; }
expect_clean() { ! grep -q 'needs attention' <<< "$summary" || fail "$name: unexpected problems: $summary"; }

entry() { printf '  - path: %s\n    owner: %s\n    issue: %s\n    expires: %s\n' "$@"; }

valid=$work/valid.yaml
{
  echo 'tests:'
  entry suite/a me https://example.com/1 "'2026-12-31'"
  entry suite-v1/a me https://example.com/3 "'2026-12-31'"
  entry other/b me https://example.com/2 2026-12-31
} > "$valid"

name="suite isolation"
setup; run "$valid" suite
expect_rc 0; expect_skipped suite/a; expect_runs suite-v1/a; expect_runs other/b; expect_clean
expect_reported '| `suite/a` | @me |'

name="suite with a shared prefix"
setup; run "$valid" suite-v1
expect_rc 0; expect_runs suite/a; expect_skipped suite-v1/a

name="all suites"
for s in . ""; do
  setup; run "$valid" "$s"
  expect_rc 0; expect_skipped suite/a; expect_skipped other/b; expect_skipped suite-v1/a
done

name="expiry"
setup; run "$valid" . 2026-12-31
expect_clean
setup; run "$valid" . 2027-01-01
expect_reported '`suite/a` expired on 2026-12-31'
expect_reported '`other/b` expired on 2026-12-31'
expect_skipped suite/a
setup; run "$valid" .
grep -q '^::error' <<< "$out" && fail "$name: error annotation before the expiry date"

name="malformed entries"
bad=$work/bad.yaml
{
  echo 'tests:'
  printf '  - path: suite/a\n    owner: me\n    expires: "2026-12-31"\n'
  entry other/b me i "'2026-02-31'"
  entry suite-v1/a me i "'2026-13-01'"
  entry missing/c me i "'2026-12-31'"
} > "$bad"
setup; run "$bad" .
expect_rc 0
expect_reported '`suite/a` is missing a path, owner, issue, or expires field.'
expect_reported '`other/b` has an invalid expires date `2026-02-31`'
expect_reported '`suite-v1/a` has an invalid expires date `2026-13-01`'
expect_reported '`missing/c` has no chainsaw-test.yaml'
grep -q '^::error' <<< "$out" || fail "$name: no error annotation"

name="unparsable file"
printf 'tests:\n  - path: [suite/a\n' > "$work/broken.yaml"
setup; run "$work/broken.yaml" .
expect_rc 0; expect_runs suite/a
expect_reported "can't be parsed, so no test is skipped"

name="missing or empty file"
for f in "$work/none.yaml" "$work/empty.yaml" "$work/no-tests.yaml" "$work/empty-list.yaml"; do
  : > "$work/empty.yaml"; echo '{}' > "$work/no-tests.yaml"; echo 'tests: []' > "$work/empty-list.yaml"
  setup; run "$f" .
  expect_rc 0; expect_runs suite/a; expect_runs other/b; expect_clean
done

name="unreadable file"
if [ "$(id -u)" != 0 ]; then
  cp "$valid" "$work/locked.yaml"; chmod 000 "$work/locked.yaml"
  setup; run "$work/locked.yaml" .
  expect_rc 1
fi

# The committed file must be well formed. Expiry is checked by the
# conformance jobs, not here, so that an expired entry doesn't block PRs.
name="test/conformance/quarantine.yaml"
# Run against a copy so the check doesn't edit the working tree.
rm -rf "$work/repo-chainsaw"; cp -R "$repo/test/conformance/chainsaw" "$work/repo-chainsaw"
: > "$work/summary"
out=$(GITHUB_STEP_SUMMARY=$work/summary "$script" "$repo/test/conformance/quarantine.yaml" \
  "$work/repo-chainsaw" . 0000-01-01 2>&1) || fail "$name: exit code $?"
summary=$(cat "$work/summary")
expect_clean

if [ "$failures" -gt 0 ]; then
  echo "$failures check(s) failed."
  exit 1
fi
echo "All quarantine.sh checks passed."
