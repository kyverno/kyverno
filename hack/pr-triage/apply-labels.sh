#!/usr/bin/env bash
# apply-labels.sh — apply the type_legacy / type_cel / type_mixed / type_shared
# labels to open PRs, based on a pr-branch-triage.py JSON report.
#
# SAFE BY DEFAULT: this script only PRINTS the `gh pr edit` commands it would
# run. It makes NO changes to any PR unless invoked with --execute.
#
# Usage:
#   ./apply-labels.sh --json pr-triage-report.json [--repo kyverno/kyverno] \
#       [--category LEGACY_ONLY,MIXED,...] [--limit N] [--execute]
#
# Examples:
#   # Dry run (default) — show what would happen for every classified PR
#   ./apply-labels.sh --json pr-triage-report.json
#
#   # Dry run for a single category
#   ./apply-labels.sh --json pr-triage-report.json --category LEGACY_ONLY
#
#   # Actually apply labels (only after maintainer review/approval of the dry run)
#   ./apply-labels.sh --json pr-triage-report.json --execute
#
# Requires: gh (authenticated), jq

set -euo pipefail

REPO="kyverno/kyverno"
JSON_FILE=""
CATEGORIES=""   # empty = all categories
LIMIT=""
EXECUTE=0

usage() { grep -E '^#( |$)' "$0" | sed 's/^# \{0,1\}//'; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO="$2"; shift 2 ;;
    --json) JSON_FILE="$2"; shift 2 ;;
    --category) CATEGORIES="$2"; shift 2 ;;
    --limit) LIMIT="$2"; shift 2 ;;
    --execute) EXECUTE=1; shift ;;
    -h|--help) usage ;;
    *) echo "unknown arg: $1" >&2; usage ;;
  esac
done

[[ -n "$JSON_FILE" ]] || { echo "error: --json <pr-triage-report.json> is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 1; }
command -v gh >/dev/null || { echo "error: gh is required" >&2; exit 1; }

if [[ -n "$CATEGORIES" ]]; then
  cat_filter=$(printf '%s\n' "${CATEGORIES//,/$'\n'}" | jq -R . | jq -s .)
else
  cat_filter='["LEGACY_ONLY","CEL_ONLY","MIXED","SHARED_ONLY"]'
fi

rows=$(jq -c --argjson cats "$cat_filter" '
  [.[] | select([.category] - $cats == [])]
' "$JSON_FILE")

count=$(jq 'length' <<<"$rows")
if [[ -n "$LIMIT" ]]; then
  rows=$(jq -c --argjson n "$LIMIT" '.[0:$n]' <<<"$rows")
  count=$(jq 'length' <<<"$rows")
fi

if [[ "$EXECUTE" -eq 0 ]]; then
  echo "### DRY RUN — no PRs will be modified. Re-run with --execute to apply labels. ###" >&2
fi
echo "Repo: $REPO   PRs matched: $count   Categories: $(jq -r 'join(",")' <<<"$cat_filter")" >&2
echo >&2

n=0
while IFS= read -r row; do
  n=$((n + 1))
  num=$(jq -r '.number' <<<"$row")
  cat=$(jq -r '.category' <<<"$row")
  label=$(jq -r '.proposed_label' <<<"$row")
  existing=$(jq -r '.labels | join(",")' <<<"$row")

  # Skip if the PR already carries the correct type_* label.
  if grep -qx "$label" <<<"${existing//,/$'\n'}"; then
    echo "[$n/$count] #$num already labeled '$label' — skip"
    continue
  fi

  # Remove any other type_* label so a PR only ever carries one classification.
  other_type_labels=$(grep -o '^type_[a-z]*' <<<"${existing//,/$'\n'}" | grep -v "^${label}\$" || true)

  cmd=(gh pr edit "$num" -R "$REPO" --add-label "$label")
  for l in $other_type_labels; do
    cmd+=(--remove-label "$l")
  done

  echo "[$n/$count] #$num ($cat): ${cmd[*]}"
  if [[ "$EXECUTE" -eq 1 ]]; then
    "${cmd[@]}"
    sleep 1   # be gentle with rate limits when applying to many PRs
  fi
done < <(jq -c '.[]' <<<"$rows")

if [[ "$EXECUTE" -eq 0 ]]; then
  echo >&2
  echo "### DRY RUN complete. Nothing was changed. Re-run with --execute after approval. ###" >&2
fi
