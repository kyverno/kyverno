#!/usr/bin/env bash
# apply-labels.sh — apply the type_legacy / type_cel / type_mixed / type_shared
# labels to open PRs, based on a pr-branch-triage.py JSON report.
#
# SAFE BY DEFAULT: this script only PRINTS the `gh label create`/`gh pr edit`
# commands it would run. It makes NO changes to any label or PR unless
# invoked with --execute.
#
# Usage:
#   ./apply-labels.sh --json pr-triage-report.json [--repo kyverno/kyverno] \
#       [--category LEGACY_ONLY,MIXED,...] [--limit N] [--force] [--execute]
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
#   # Overwrite existing conflicting type_* labels (e.g. forced re-triage)
#   ./apply-labels.sh --json pr-triage-report.json --force --execute
#
# Requires: gh (authenticated), jq

set -euo pipefail

REPO="kyverno/kyverno"
JSON_FILE=""
CATEGORIES=""   # empty = all categories
LIMIT=""
FORCE=0
EXECUTE=0

usage() { grep -E '^#( |$)' "$0" | sed 's/^# \{0,1\}//'; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO="$2"; shift 2 ;;
    --json) JSON_FILE="$2"; shift 2 ;;
    --category) CATEGORIES="$2"; shift 2 ;;
    --limit) LIMIT="$2"; shift 2 ;;
    --force|--overwrite) FORCE=1; shift ;;
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
  [.[] | select([.category] - $cats == []) | select(.proposed_label != null)]
' "$JSON_FILE")

count=$(jq 'length' <<<"$rows")
if [[ -n "$LIMIT" ]]; then
  rows=$(jq -c --argjson n "$LIMIT" '.[0:$n]' <<<"$rows")
  count=$(jq 'length' <<<"$rows")
fi

if [[ "$EXECUTE" -eq 0 ]]; then
  echo "### DRY RUN — no labels or PRs will be modified. Re-run with --execute to apply. ###" >&2
fi
echo "Repo: $REPO   PRs matched: $count   Categories: $(jq -r 'join(",")' <<<"$cat_filter")" >&2
echo >&2

# ---------------------------------------------------------------------------
# 1. Ensure the four type_* labels exist. `gh pr edit --add-label` fails if a
#    label hasn't been created yet, and the labeling step must not silently
#    depend on someone having pre-created them via .github/labels.yml. This
#    is itself dry-run gated: only creates labels when --execute is passed.
# ---------------------------------------------------------------------------
# NOTE: intentionally avoids `declare -A` (associative arrays) so this script
# still runs under bash 3.2 (macOS's default /bin/bash).
label_color() {
  case "$1" in
    type_legacy) echo "b60205" ;;
    type_cel)    echo "00BCD4" ;;
    type_mixed)  echo "FBCA04" ;;
    type_shared) echo "c5def5" ;;
  esac
}
label_desc() {
  case "$1" in
    type_legacy) echo "Legacy kyverno.io ClusterPolicy/Policy/CleanupPolicy PR — candidate to retarget to release-1.19" ;;
    type_cel)    echo "CEL policies.kyverno.io PR — stays on main" ;;
    type_mixed)  echo "Touches both legacy and CEL policy code — needs manual split/keep decision" ;;
    type_shared) echo "Touches neither legacy nor CEL policy code (shared infra/docs/deps/CI)" ;;
  esac
}

existing_labels=$(gh label list -R "$REPO" --limit 1000 --json name -q '.[].name')
for label in type_legacy type_cel type_mixed type_shared; do
  if grep -qx "$label" <<<"$existing_labels"; then
    continue
  fi
  cmd=(gh label create "$label" -R "$REPO" --color "$(label_color "$label")" --description "$(label_desc "$label")")
  echo "[labels] ${cmd[*]}"
  if [[ "$EXECUTE" -eq 1 ]]; then
    "${cmd[@]}"
  fi
done
echo >&2

# ---------------------------------------------------------------------------
# 2. Apply labels per PR. Idempotence is based on each PR's LIVE GitHub
#    labels (fetched here), not the labels captured in the (possibly stale)
#    triage JSON snapshot — a maintainer may have relabeled a PR, or run the
#    triage script again with different results, since the report was
#    generated.
# ---------------------------------------------------------------------------
n=0
while IFS= read -r row; do
  n=$((n + 1))
  num=$(jq -r '.number' <<<"$row")
  cat=$(jq -r '.category' <<<"$row")
  label=$(jq -r '.proposed_label' <<<"$row")

  if ! existing=$(gh pr view "$num" -R "$REPO" --json labels -q '.labels[].name' 2>/dev/null); then
    echo "[$n/$count] #$num: failed to fetch live labels from GitHub — skipping" >&2
    continue
  fi

  # Identify any existing type_* labels currently attached to the PR.
  existing_type_labels=$(grep -o '^type_[a-z_]*' <<<"$existing" | sort -u || true)

  # Case 1: PR already has exactly the proposed label and no other type_* labels.
  if [[ "$existing_type_labels" == "$label" ]]; then
    echo "[$n/$count] #$num already labeled '$label' (live) — skip"
    continue
  fi

  # Case 2: PR has a different type_* label (e.g. manually set by maintainer).
  # By default, preserve the manual label unless --force is specified.
  if [[ -n "$existing_type_labels" ]] && grep -qvx "$label" <<<"$existing_type_labels" && [[ "$FORCE" -eq 0 ]]; then
    echo "[$n/$count] #$num has existing type label '$(tr '\n' ' ' <<<"$existing_type_labels" | xargs)' (differs from '$label') — skipping to preserve manual triage (pass --force to overwrite)"
    continue
  fi

  # Remove any conflicting type_* labels so a PR only ever carries one classification.
  other_type_labels=$(grep -v "^${label}\$" <<<"$existing_type_labels" || true)

  cmd=(gh pr edit "$num" -R "$REPO" --add-label "$label")
  for l in $other_type_labels; do
    if [[ -n "$l" ]]; then
      cmd+=(--remove-label "$l")
    fi
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
