---
description: Find open PRs that are behind the base branch and automatically trigger a branch update.
on:
  schedule: every 1h
engine:
  id: copilot
permissions:
  contents: read
  pull-requests: read
tools:
  github:
    toolsets: [pull_requests]
steps:
  - name: List open pull requests
    env:
      GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    run: |
      set -euo pipefail
      mkdir -p /tmp/gh-aw/agent
      gh api --paginate "repos/${{ github.repository }}/pulls?state=open&per_page=100" \
        | jq -s 'add | map({number, title, draft, head_ref: .head.ref, base_ref: .base.ref})' \
        > /tmp/gh-aw/agent/open-prs.json
      COUNT=$(jq length /tmp/gh-aw/agent/open-prs.json)
      echo "Found $COUNT open PRs"
      jq -r '.[] | "  #\(.number) [\(if .draft then "draft" else "open" end)] \(.head_ref) -> \(.base_ref): \(.title)"' \
        /tmp/gh-aw/agent/open-prs.json
safe-outputs:
  noop: {}
  jobs:
    update-pr-branch:
      description: "Trigger a branch update for one or more pull requests that are behind their base branch. Call this once after identifying all out-of-date PRs."
      runs-on: ubuntu-latest
      permissions:
        contents: write
        pull-requests: write
      inputs:
        pull_numbers:
          description: "Comma-separated list of PR numbers to update (e.g. 12,34,56)"
          required: true
          type: string
      steps:
        - name: Update PR branches via GitHub API
          env:
            GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
            REPO: ${{ github.repository }}
            GITHUB_SERVER_URL: ${{ github.server_url }}
          run: |
            GH_HOST="${GITHUB_SERVER_URL#https://}"
            GH_HOST="${GH_HOST#http://}"
            export GH_HOST
            if [ ! -f "$GH_AW_AGENT_OUTPUT" ]; then
              echo "No agent output found"
              exit 1
            fi

            PR_NUMBERS_CSV=$(jq -r '.items[] | select(.type == "update_pr_branch") | .pull_numbers' "$GH_AW_AGENT_OUTPUT" | head -1)

            if [ -z "$PR_NUMBERS_CSV" ]; then
              echo "No PRs to update"
              exit 0
            fi

            UPDATED=0
            FAILED=0

            IFS=',' read -ra PR_LIST <<< "$PR_NUMBERS_CSV"
            for PR_NUMBER in "${PR_LIST[@]}"; do
              PR_NUMBER=$(echo "$PR_NUMBER" | tr -d ' ')
              [ -z "$PR_NUMBER" ] && continue

              echo "Updating branch for PR #$PR_NUMBER..."

              RESPONSE_FILE=$(mktemp)
              gh api \
                --include \
                --method PUT \
                -H "Accept: application/vnd.github+json" \
                "/repos/$REPO/pulls/$PR_NUMBER/update-branch" \
                > "$RESPONSE_FILE" 2>&1
              EXIT_CODE=$?

              STATUS_CODE=$(grep -m1 -E '^HTTP/' "$RESPONSE_FILE" | awk '{print $2}')
              RESPONSE_BODY=$(awk 'BEGIN { body=0 } body { print } /^(\r)?$/ { body=1 }' "$RESPONSE_FILE")

              if [ -n "$RESPONSE_BODY" ] && echo "$RESPONSE_BODY" | jq -e . >/dev/null 2>&1; then
                RESPONSE_MESSAGE=$(echo "$RESPONSE_BODY" | jq -r '.message // empty')
              else
                RESPONSE_MESSAGE=$(cat "$RESPONSE_FILE")
              fi

              if [ $EXIT_CODE -eq 0 ] && echo "$RESPONSE_MESSAGE" | grep -qi "scheduled"; then
                echo "  ✓ PR #$PR_NUMBER — branch update scheduled"
                UPDATED=$((UPDATED + 1))
              elif [ $EXIT_CODE -eq 0 ] && echo "$RESPONSE_MESSAGE" | grep -qi "already up"; then
                echo "  - PR #$PR_NUMBER — already up-to-date, skipping"
              else
                echo "  ✗ PR #$PR_NUMBER — update failed (exit $EXIT_CODE, http ${STATUS_CODE:-unknown}): $RESPONSE_MESSAGE"
                FAILED=$((FAILED + 1))
              fi

              rm -f "$RESPONSE_FILE"
            done

            echo ""
            echo "Summary: $UPDATED updated, $FAILED failed"
            if [ $FAILED -gt 0 ]; then
              exit 1
            fi
---

# PR Branch Auto-Updater

You are an AI agent that keeps pull request branches up-to-date with the base branch.

## Mission

Identify open, non-draft pull requests whose branches have fallen behind their base branch, then trigger a branch update for each one via the `update_pr_branch` safe output.

## Current Context

- **Repository**: ${{ github.repository }}
- **Open PRs pre-fetched to**: `/tmp/gh-aw/agent/open-prs.json`

## Task

### 1. Read the Pre-Fetched PR List

Read the list of open PRs that was pre-fetched for you:

```text
Read /tmp/gh-aw/agent/open-prs.json
Fields available: number, title, draft, head_ref, base_ref
```

### 2. Fetch Full Details for Each Non-Draft PR

For each PR where `draft` is `false`, fetch the full PR record to get an accurate `mergeable_state`:

```text
Use pull_request_read with the PR number to get the full PR object.
Key field: mergeable_state — "behind" | "dirty" | "clean" | "blocked" | "unknown"
```

> **Important**: `mergeable_state` is only reliably computed by GitHub when a PR is fetched
> individually. The bulk list endpoint returns `"unknown"` for most PRs — always re-fetch.

### 3. Classify Each PR

| `mergeable_state` | Action |
| --- | --- |
| `"behind"` | ✅ **Needs update** — add to the update list |
| `"dirty"` | ⚠️ Skip — merge conflicts, requires human attention |
| `"clean"` / `"blocked"` | ✅ Skip — already up-to-date or waiting on reviews |
| `"unknown"` | ⏭️ Skip — state not yet computed |

Also skip any PR where `draft: true`.

### 4. Trigger Branch Updates

Collect **all** PR numbers that need an update and call `update_pr_branch` **once** with a comma-separated list:

```text
update_pr_branch(pull_numbers="12,34,56")
```

## Guidelines

- **Fetch each PR individually** to get reliable `mergeable_state` — the pre-fetched list gives you PR numbers to iterate, not final state.
- **Never modify code** — this workflow only triggers GitHub's built-in branch-update mechanism.
- **Batch the output call** — call `update_pr_branch` a single time with all applicable PR numbers, not once per PR.
- **Be thorough** — check every non-draft open PR before concluding.

## Error Handling

- If fetching details for a PR fails (e.g. API error), skip that PR, note the failure, and continue with the rest. Do not abort the entire run for a single failure.
- If the pre-fetched file is empty or missing, use the GitHub pull request tools to list open PRs directly.

## Safe Outputs

After completing your analysis:

- **If one or more PRs are behind**: call `update_pr_branch` once with `pull_numbers` as a comma-separated string of all PR numbers that need updating (e.g. `"12,34,56"`).
- **If no PRs are behind**: call `noop` with a message such as `"No action needed: all N open PRs are up-to-date with their base branch."`.
