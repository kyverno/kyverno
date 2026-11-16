#!/usr/bin/env bash
# Skips the quarantined conformance tests that fall under one test suite.
#
# Usage: quarantine.sh <quarantine-file> <chainsaw-root> <tests-path> [today]
#
# - quarantine-file: test/conformance/quarantine.yaml
# - chainsaw-root:   test/conformance/chainsaw
# - tests-path:      the suite this job runs, relative to chainsaw-root, or "."
# - today:           YYYY-MM-DD; defaults to the current UTC date (for tests)
#
# For every entry under tests-path, sets spec.skip in its chainsaw-test.yaml.
# Reports malformed and expired entries as error annotations and in the job
# summary, but doesn't fail: a broken entry must not turn every PR's E2E Gate
# red. It exits non-zero only when it can't read the quarantine file.
set -euo pipefail

file=$1
root=$2
suite=${3:-.}
today=${4:-$(date -u +%F)}
summary=${GITHUB_STEP_SUMMARY:-/dev/null}

suite=${suite#./}
suite=${suite%/}

if [ ! -f "$file" ]; then
  echo "No quarantine file at $file, so no test is skipped."
  exit 0
fi

if [ ! -r "$file" ]; then
  echo "::error file=$file::Can't read the quarantine file $file."
  exit 1
fi

# Succeeds only for a real calendar date in YYYY-MM-DD form. Uses GNU date on
# the runners, and BSD date when you run the tests on macOS.
valid_date() {
  [[ "$1" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || return 1
  local parsed
  parsed=$(date -u -d "$1" +%F 2>/dev/null || date -u -j -f %Y-%m-%d "$1" +%F 2>/dev/null) || return 1
  [ "$parsed" = "$1" ]
}

skipped=()
problems=()

# A file that isn't valid YAML skips nothing and is reported below, so that it
# doesn't fail every conformance job.
errors=$(mktemp)
trap 'rm -f "$errors"' EXIT
if ! entries=$(yq -r '(.tests // [])[] | [.path // "", .owner // "", .issue // "", .expires // ""] | join("|")' "$file" 2> "$errors"); then
  reason=$(tr '\n' ' ' < "$errors")
  problems+=("\`$file\` can't be parsed, so no test is skipped: $reason")
  echo "::error file=$file::Can't parse the quarantine file, so no test is skipped: $reason"
  entries=
fi

while IFS='|' read -r path owner issue expires; do
  [ -z "$path$owner$issue$expires" ] && continue
  path=${path#./}
  path=${path%/}
  case "$suite" in
    "" | .) ;;
    *) [ "$path" = "$suite" ] || [[ "$path" == "$suite"/* ]] || continue ;;
  esac

  if [ -z "$path" ] || [ -z "$owner" ] || [ -z "$issue" ] || [ -z "$expires" ]; then
    problems+=("\`${path:-<no path>}\` is missing a path, owner, issue, or expires field.")
    echo "::error file=$file::Quarantine entry ${path:-<no path>} is missing a path, owner, issue, or expires field."
  fi
  if ! valid_date "$expires"; then
    problems+=("\`$path\` has an invalid expires date \`$expires\`; use a quoted YYYY-MM-DD string.")
    echo "::error file=$file::Quarantine entry $path has an invalid expires date '$expires'."
  elif [[ "$expires" < "$today" ]]; then
    problems+=("\`$path\` expired on $expires. Owner: @$owner. Fix the test and remove the entry, or extend it with a reason in $issue.")
    echo "::error file=$file::Quarantine of $path expired on $expires (owner: $owner, issue: $issue)."
  fi

  test_file="$root/$path/chainsaw-test.yaml"
  if [ ! -f "$test_file" ]; then
    problems+=("\`$path\` has no chainsaw-test.yaml, so nothing is skipped. Remove or fix the entry.")
    echo "::error file=$file::Quarantine entry $path has no $test_file."
    continue
  fi
  yq eval '.spec.skip = true' -i "$test_file"
  echo "Skipped $path (owner: $owner, issue: $issue, expires: $expires)."
  skipped+=("| \`$path\` | @$owner | $issue | $expires |")
done <<< "$entries"

{
  if [ ${#problems[@]} -gt 0 ]; then
    echo "### :warning: Quarantine file needs attention"
    echo
    for p in "${problems[@]}"; do echo "- $p"; done
    echo
  fi
  if [ ${#skipped[@]} -gt 0 ]; then
    echo "### Quarantined tests skipped in this job"
    echo
    echo "| Test | Owner | Issue | Expires |"
    echo "| --- | --- | --- | --- |"
    printf '%s\n' "${skipped[@]}"
    echo
  fi
} >> "$summary"

if [ ${#skipped[@]} -eq 0 ]; then
  echo "No quarantined tests under $suite."
fi
