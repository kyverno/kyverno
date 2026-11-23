#!/usr/bin/env bash
# Tests normalize-key.sh. Needs bash and openssl.
#
# Usage: .github/actions/workflow/app-token/normalize-key_test.sh
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

failures=0
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

openssl genrsa -out "$work/key.pem" 2048 2>/dev/null
pem=$(cat "$work/key.pem")
lines=$(grep -c . "$work/key.pem")

# Runs normalize-key.sh with the given key; sets rc, out, and output.
run() {
  : > "$work/output"
  rc=0
  out=$(GITHUB_OUTPUT="$work/output" GITHUB_REPOSITORY=kyverno/kyverno RAW_APP_PRIVATE_KEY="$1" \
    "$here/normalize-key.sh" 2>&1) || rc=$?
  output=$(cat "$work/output")
}

expect_key() {
  local name=$1
  [ "$rc" = 0 ] || { fail "$name: exit code $rc: $out"; return; }
  expected=$(printf 'private_key<<EOF\n%s\nEOF\nrepository=kyverno' "$pem")
  [ "$output" = "$expected" ] || fail "$name: unexpected output: $output"
  masks=$(grep -c '^::add-mask::' <<< "$out" || true)
  [ "$masks" = "$lines" ] || fail "$name: masked $masks lines, want $lines"
  while IFS= read -r line; do
    grep -qxF "::add-mask::$line" <<< "$out" || fail "$name: line not masked: $line"
  done < "$work/key.pem"
}

run "$pem"
expect_key PEM

run "$(awk 'NR > 1 { printf "\\n" } { printf "%s", $0 }' "$work/key.pem")"
expect_key "escaped newlines"

run "$(base64 < "$work/key.pem" | tr -d '\n')"
expect_key base64

run "$(base64 < "$work/key.pem")"
expect_key "wrapped base64"

for bad in "not a key" "$(printf 'not a key' | base64)"; do
  run "$bad"
  [ "$rc" != 0 ] || fail "invalid key '$bad' was accepted"
  [ -z "$output" ] || fail "invalid key '$bad' wrote output: $output"
  grep -q 'does not look like a valid PEM private key' <<< "$out" || fail "invalid key '$bad': no error message"
done

if [ "$failures" -gt 0 ]; then
  echo "$failures failure(s)"
  exit 1
fi
echo "All tests passed."
