#!/usr/bin/env bash
# Print the conformance suites a set of changed files needs, or ALL.
# Usage: scripts/conformance-suites-for.sh <file>...
#        git diff --name-only main... | scripts/conformance-suites-for.sh
#        scripts/conformance-suites-for.sh --verify
#
# Reads .github/conformance-suite-map.yaml. A file matching nothing in the map
# prints ALL, so an incomplete map costs CI time rather than coverage.

# -f matters: the map holds globs, and without it the shell expands them
# against the working tree instead of matching them as patterns.
set -ef

MAP=".github/conformance-suite-map.yaml"
CHAINSAW_DIR="test/conformance/chainsaw"
WORKFLOW=".github/workflows/tests-conformance.yaml"

if [ ! -f "$MAP" ]; then
  echo "error: $MAP not found; run from the repository root" >&2
  exit 1
fi

# Without this the yq calls return nothing and every check silently passes.
if ! command -v yq >/dev/null 2>&1; then
  echo "error: yq is required (.github/actions/tools/yq installs it in CI)" >&2
  exit 1
fi

# Bash patterns already let * span /, so ** collapses to * and a leading */
# is redundant.
to_pattern() {
  local p="${1//\*\*/\*}"
  printf '%s' "${p#\*/}"
}

runnable_suites() {
  {
    grep -o -- 'tests-path:[[:space:]]*[^[:space:]]*' "$WORKFLOW" | sed 's|tests-path:[[:space:]]*||'
    grep -o -- "$CHAINSAW_DIR/[A-Za-z0-9_.-]*" "$WORKFLOW" | sed "s|^$CHAINSAW_DIR/||"
  } | cut -d/ -f1 | grep -v '^\.$' | sort -u
}

# glob<TAB>suite, one pair per line.
pairs=$(yq -r '.suites | to_entries[] | .key as $k | .value[] | $k + "\t" + .' "$MAP")
skip_globs=$(yq -r '.skip[]' "$MAP")

if [ "${1:-}" = "--verify" ]; then
  failed=0
  runnable=$(runnable_suites)
  while IFS=$'\t' read -r glob suite; do
    [ -n "$suite" ] || continue
    if [ ! -d "$CHAINSAW_DIR/$suite" ]; then
      echo "error: $MAP maps $glob to '$suite', which is not a directory in $CHAINSAW_DIR" >&2
      failed=1
    elif ! printf '%s\n' "$runnable" | grep -qx -- "$suite"; then
      echo "error: $MAP maps $glob to '$suite', which no job in $WORKFLOW runs" >&2
      failed=1
    fi
  done <<EOF
$pairs
EOF
  while IFS= read -r key; do
    case "$key" in
      skip|suites|"") ;;
      *) echo "error: $MAP has unknown top level key '$key'" >&2; failed=1 ;;
    esac
  done <<EOF
$(yq -r 'keys[]' "$MAP")
EOF
  if [ "$failed" -ne 0 ]; then
    exit 1
  fi
  echo "ok: $MAP is consistent with $WORKFLOW"
  exit 0
fi

if [ "$#" -gt 0 ]; then
  changed=$(printf '%s\n' "$@")
else
  changed=$(cat)
fi

selected=""
while IFS= read -r file; do
  [ -n "$file" ] || continue

  # A change inside a suite needs that suite, no map entry required.
  case "$file" in
    "$CHAINSAW_DIR"/*)
      rest=${file#"$CHAINSAW_DIR"/}
      selected="$selected ${rest%%/*}"
      continue
      ;;
  esac

  skipped=false
  while IFS= read -r glob; do
    [ -n "$glob" ] || continue
    pat=$(to_pattern "$glob")
    case "$file" in
      $pat) skipped=true; break ;;
    esac
  done <<EOF
$skip_globs
EOF
  if [ "$skipped" = true ]; then
    continue
  fi

  matched=false
  while IFS=$'\t' read -r glob suite; do
    [ -n "$suite" ] || continue
    pat=$(to_pattern "$glob")
    case "$file" in
      $pat) matched=true; selected="$selected $suite" ;;
    esac
  done <<EOF
$pairs
EOF

  # Unmapped path: fall back to the full run.
  if [ "$matched" = false ]; then
    echo ALL
    exit 0
  fi
done <<EOF
$changed
EOF

printf '%s\n' $selected | sort -u | tr '\n' ' ' | sed 's/ *$//'
echo
