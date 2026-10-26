#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

case "$EVENT_NAME" in
  pull_request_target)
    # Same eligibility as the sweep: non-draft, not Dependabot's (dependabot-merge-triage.yaml owns those).
    if [[ "$PR_AUTHOR" == "dependabot[bot]" ]]; then
      echo '[]' > qualifying-numbers.json
    else
      echo "[$PR_NUMBER]" > qualifying-numbers.json
    fi
    ;;
  schedule|workflow_dispatch)
    echo 'sweep' > mode.txt
    ;;
  *)
    echo "::error::Unsupported readiness event: $EVENT_NAME"
    exit 1
    ;;
esac

if [[ -f mode.txt ]]; then
  mode=sweep
else
  mode=single
  echo "Processing PR(s): $(cat qualifying-numbers.json)"
fi


if [[ "$mode" == "sweep" ]]; then
# Cheap fields only — the full payload for all ~300+ open PRs times out the GraphQL API.
query='
  query($search: String!, $after: String) {
    search(query: $search, type: ISSUE, first: 100, after: $after) {
      pageInfo { hasNextPage endCursor }
      nodes {
        ... on PullRequest {
          number
          isDraft
          closingIssuesReferences(first: 100) {
            pageInfo { hasNextPage endCursor }
            nodes { repository { nameWithOwner } milestone { title state } }
          }
        }
      }
    }
  }'
search="repo:${GITHUB_REPOSITORY} is:pr is:open draft:false -author:app/dependabot"

# milestone-pr: closes a same-repo issue with an open "Kyverno Release ..." milestone. Only gates
# that label — every open PR found above gets a readiness check, milestone or not.
qualifies='
  [.closingIssuesReferences.nodes[]?
    | select(.repository.nameWithOwner == $repo
        and .milestone != null
        and .milestone.state == "OPEN"
        and ($prefix != "")
        and (.milestone.title | startswith($prefix)))]
  | length > 0'

echo '[]' > qualifying-numbers.json
echo '[]' > milestone-numbers.json
after=""
page=0
while :; do
  page=$((page + 1))
  if [[ -z "$after" ]]; then
    gh api graphql -f query="$query" -f search="$search" > response.json
  else
    gh api graphql -f query="$query" -f search="$search" -f after="$after" > response.json
  fi

  jq -e '.data.search | (.nodes | type == "array") and
    (.pageInfo.hasNextPage | type == "boolean")' response.json > /dev/null
  jq -c '[.data.search.nodes[] | select(.isDraft | not) | .number]' \
    response.json > page-numbers.json
  jq -s 'add' qualifying-numbers.json page-numbers.json > numbers-next.json \
    && mv numbers-next.json qualifying-numbers.json

  jq -c --arg repo "${GITHUB_REPOSITORY}" --arg prefix "$MILESTONE_PREFIX" \
    "[.data.search.nodes[] | select(.isDraft | not) | select($qualifies) | .number]" \
    response.json > page-milestone-numbers.json
  jq -s 'add' milestone-numbers.json page-milestone-numbers.json > milestone-next.json \
    && mv milestone-next.json milestone-numbers.json

  # A PR closing more than 100 issues isn't realistic, but flag it loudly rather than silently miss it.
  still_truncated=$(jq --arg repo "${GITHUB_REPOSITORY}" --arg prefix "$MILESTONE_PREFIX" \
    "[.data.search.nodes[] | select(.isDraft | not) | select(.closingIssuesReferences.pageInfo.hasNextPage and ($qualifies | not)) | .number]" \
    response.json)
  [[ "$still_truncated" != "[]" ]] && echo "::warning::PR(s) $still_truncated close more than 100 issues and may be wrongly excluded from milestone-pr."

  has_next=$(jq -r '.data.search.pageInfo.hasNextPage' response.json)
  [[ "$has_next" != "true" ]] && break
  after=$(jq -r '.data.search.pageInfo.endCursor' response.json)
  [[ -n "$after" && "$after" != "null" ]] || { echo "::error::Missing discovery cursor"; exit 1; }

  if [[ "$page" -ge 20 ]]; then
    echo "::error::Discovery still truncated after 20 pages (2000 PRs)."
    exit 1
  fi
done
echo "Found $(jq 'length' qualifying-numbers.json) open PR(s) to evaluate."
echo "Found $(jq 'length' milestone-numbers.json) milestone PR(s) to stamp."

# Stamp milestone-pr on qualifying PRs. Removing it from PRs that stop qualifying is a follow-up.
for number in $(jq -r '.[]' milestone-numbers.json); do
  current_labels=$(gh pr view "$number" --repo "${GITHUB_REPOSITORY}" --json labels --jq '[.labels[].name]')
  has_milestone_label=$(jq 'any(. == "milestone-pr")' <<< "$current_labels")
  if [[ "$has_milestone_label" == "false" ]]; then
    gh pr edit "$number" --repo "${GITHUB_REPOSITORY}" --add-label "milestone-pr"
  fi
done
fi


set -euo pipefail

detail_query='
  query($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) {
      pullRequest(number: $number) {
        number
        state
        isDraft
        author { login }
        labels(first: 100) { pageInfo { hasNextPage endCursor } nodes { name } }
        commits(last: 1) {
          nodes { commit {
            oid
            statusCheckRollup { contexts(first: 100) { pageInfo { hasNextPage endCursor } nodes {
              ... on CheckRun { name conclusion status }
              ... on StatusContext { context state }
            } } }
          } }
        }
        reviewThreads(first: 100) {
          pageInfo { hasNextPage endCursor }
          nodes { isResolved comments(last: 1) { nodes { author { login } createdAt } } }
        }
      }
    }
  }'
# Continuation queries for the three connections above, used to finish any that didn't fit in 100.
labels_page_query='
  query($owner: String!, $name: String!, $number: Int!, $after: String!) {
    repository(owner: $owner, name: $name) { pullRequest(number: $number) {
      labels(first: 100, after: $after) { pageInfo { hasNextPage endCursor } nodes { name } }
    } }
  }'
checks_page_query='
  query($owner: String!, $name: String!, $number: Int!, $after: String!) {
    repository(owner: $owner, name: $name) { pullRequest(number: $number) {
      commits(last: 1) { nodes { commit {
        statusCheckRollup { contexts(first: 100, after: $after) { pageInfo { hasNextPage endCursor } nodes {
          ... on CheckRun { name conclusion status }
          ... on StatusContext { context state }
        } } }
      } } }
    } }
  }'
threads_page_query='
  query($owner: String!, $name: String!, $number: Int!, $after: String!) {
    repository(owner: $owner, name: $name) { pullRequest(number: $number) {
      reviewThreads(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes { isResolved comments(last: 1) { nodes { author { login } createdAt } } }
      }
    } }
  }'
owner="${GITHUB_REPOSITORY%/*}"
name="${GITHUB_REPOSITORY#*/}"

# Paginates one connection on $pr_file to completion, given its jq path and *_page_query. 20-page cap
# as a last-resort safety valve; the truncated-connection handling below still covers that case.
paginate_connection() {
  local pr_file="$1" conn_path="$2" query="$3" extra_page=0
  validate_connection "$pr_file" "$conn_path" || return 1
  while [[ "$(jq -r "${conn_path}.pageInfo.hasNextPage" "$pr_file")" == "true" ]]; do
    extra_page=$((extra_page + 1))
    if [[ "$extra_page" -gt 20 ]]; then
      echo "::warning::$conn_path on PR in $pr_file still has more pages after 20 extra fetches — leaving it truncated."
      break
    fi
    after=$(jq -r "${conn_path}.pageInfo.endCursor" "$pr_file")
    [[ -n "$after" && "$after" != "null" ]] || return 1
    gh api graphql -f query="$query" -f owner="$owner" -f name="$name" -F number="$number" -f after="$after" > page.json
    validate_connection page.json ".data.repository.pullRequest${conn_path}" || return 1
    jq --argjson extra "$(jq -c ".data.repository.pullRequest${conn_path}.nodes" page.json)" \
      "${conn_path}.nodes += \$extra" "$pr_file" > pr-next.json && mv pr-next.json "$pr_file"
    jq --argjson pi "$(jq -c ".data.repository.pullRequest${conn_path}.pageInfo" page.json)" \
      "${conn_path}.pageInfo = \$pi" "$pr_file" > pr-next.json && mv pr-next.json "$pr_file"
  done
}

# Written straight to a file, never through a bash variable — PR content can contain arbitrary text.
: > pr-details.jsonl
for number in $(jq -r '.[]' qualifying-numbers.json); do
  gh api graphql -f query="$detail_query" -f owner="$owner" -f name="$name" -F number="$number" \
    | jq -c '.data.repository.pullRequest' > one-pr.json

  # A qualifying PR can close between discovery and here — skip rather than process a null result.
  if [[ "$(jq -r 'type' one-pr.json)" != "object" ]]; then
    echo "PR #$number: detail fetch returned nothing (closed mid-run?) — skipping."
    continue
  fi
  [[ "$(jq -r '.state' one-pr.json)" == "OPEN" ]] || continue
  [[ "$(jq -r '.author.login' one-pr.json)" == "dependabot[bot]" ]] && continue
  if [[ "$(jq -r '.isDraft' one-pr.json)" == "true" ]]; then
    # A converted draft must not retain readiness; do not evaluate or comment on it.
    for label in ready-for-review needs-author-action workflow-approval-required; do
      gh pr edit "$number" --repo "$GITHUB_REPOSITORY" --remove-label "$label"
    done
    continue
  fi

  paginate_connection one-pr.json ".labels" "$labels_page_query"
  # A missing rollup is pending CI, not a proven empty/successful connection.
  if [[ "$(jq -r '.commits.nodes[0].commit.statusCheckRollup' one-pr.json)" != "null" ]]; then
    paginate_connection one-pr.json ".commits.nodes[0].commit.statusCheckRollup.contexts" "$checks_page_query"
  fi
  paginate_connection one-pr.json ".reviewThreads" "$threads_page_query"

  cat one-pr.json >> pr-details.jsonl
done
jq -s '.' pr-details.jsonl > prs.json
returned_count=$(jq 'length' prs.json)

for ((i = 0; i < returned_count; i++)); do
  jq -c ".[$i]" prs.json > pr.json

  number=$(jq -r '.number' pr.json)
  author=$(jq -r '.author.login' pr.json)
  findings=$(jq -c -f readiness-checks.jq pr.json)
  # infra_checks aren't the author's to fix — FYI comment below, not a reason to withhold ready-for-review.
  blocking=$(jq -c '[.[] | select(.id != "infra_checks")]' <<< "$findings")
  infra_finding=$(jq -c '[.[] | select(.id == "infra_checks")]' <<< "$findings")
  has_label=$(jq -r '[.labels.nodes[].name] | any(. == "needs-author-action")' pr.json)
  has_ready_label=$(jq -r '[.labels.nodes[].name] | any(. == "ready-for-review")' pr.json)
  has_approval_label=$(jq -r '[.labels.nodes[].name] | any(. == "workflow-approval-required")' pr.json)

  # Gated workflows are absent from the rollup, so the PR would look clean. Matched by head SHA
  # (`pull_requests` is empty on fork runs); on lookup failure, skip the PR.
  head_sha=$(jq -r '.commits.nodes[0].commit.oid' pr.json)
  if [[ ! "$head_sha" =~ ^[0-9a-f]{40,64}$ ]]; then
    echo "::warning::PR #$number: missing head SHA — leaving labels as-is."
    continue
  fi
  if ! gated_count=$(gh api "repos/${GITHUB_REPOSITORY}/actions/runs?head_sha=${head_sha}&status=action_required&per_page=1" --jq '.total_count') \
    || [[ ! "$gated_count" =~ ^[0-9]+$ ]]; then
    echo "::warning::PR #$number: could not check for workflow runs awaiting approval — leaving labels as-is."
    continue
  fi
  gated=false
  [[ "$gated_count" -gt 0 ]] && gated=true

  # A truncated connection means "no findings" is unproven — leave labels as-is rather than clear them.
  truncated=$(jq -r '
    [.labels.pageInfo.hasNextPage,
     (.commits.nodes[0].commit.statusCheckRollup.contexts.pageInfo.hasNextPage // false),
     .reviewThreads.pageInfo.hasNextPage]
    | any' pr.json)

  # A check still running isn't a finding — don't mark ready until it settles. No check data at all
  # (rollup absent, or empty — e.g. right after a PR opens) counts as pending too, not "clean."
  pending=$(jq -r '
    (.commits.nodes[0].commit.statusCheckRollup.contexts.nodes | length == 0) or
    ([.commits.nodes[0].commit.statusCheckRollup.contexts.nodes[]?
      | select((.name == null and .context == null) or
          (.status == null and .state == null) or
          (.status == "COMPLETED" and .conclusion == null) or
          (.status? and .status != "COMPLETED") or .state? == "PENDING" or .state? == "EXPECTED")]
     | length > 0)' pr.json)

  # Do not clear labels or report readiness for data from a head that changed during pagination.
  gh api "repos/${GITHUB_REPOSITORY}/pulls/$number" > live-pr.json
  if ! jq -e --arg head "$head_sha" \
    '.state == "open" and .draft == false and .head.sha == $head' live-pr.json > /dev/null; then
    echo "::warning::PR #$number: state/draft/head changed during evaluation — leaving labels as-is."
    continue
  fi

  if [[ "$truncated" == "true" ]]; then
    # Never clear author-action/approval labels based on a partial connection.
    [[ "$has_ready_label" == "true" ]] && gh pr edit "$number" --repo "$GITHUB_REPOSITORY" --remove-label ready-for-review
    echo "::warning::PR #$number: truncated connection — not evaluating readiness."
    continue
  fi

  if [[ "$gated" == "true" && "$has_approval_label" == "false" ]]; then
    gh pr edit "$number" --repo "${GITHUB_REPOSITORY}" --add-label "workflow-approval-required"
  elif [[ "$gated" == "false" && "$has_approval_label" == "true" ]]; then
    gh pr edit "$number" --repo "${GITHUB_REPOSITORY}" --remove-label "workflow-approval-required"
  fi

  if [[ "$blocking" == "[]" ]]; then
    if [[ "$gated" == "true" ]]; then
      # Not the author's to fix, but never ready while CI hasn't run.
      [[ "$has_ready_label" == "true" ]] && gh pr edit "$number" --repo "${GITHUB_REPOSITORY}" --remove-label "ready-for-review"
      [[ "$has_label" == "true" ]] && gh pr edit "$number" --repo "${GITHUB_REPOSITORY}" --remove-label "needs-author-action"
      echo "PR #$number: $gated_count workflow run(s) awaiting approval — not ready-for-review."
    elif [[ "$pending" == "true" ]]; then
      # A new commit's checks aren't settled yet — don't let an old ready-for-review keep claiming so.
      [[ "$has_ready_label" == "true" ]] && gh pr edit "$number" --repo "${GITHUB_REPOSITORY}" --remove-label "ready-for-review"
      echo "PR #$number: checks still pending — removed ready-for-review if present."
    else
      if [[ "$infra_finding" != "[]" ]]; then
        infra_message=$(jq -r '.[0].message' <<< "$infra_finding")
        infra_marker="<!-- pr-readiness-check-infra: $(printf '%s' "$infra_message" | sha1sum | cut -c1-12) -->"
        gh api "repos/${GITHUB_REPOSITORY}/issues/$number/comments" --paginate --jq '.[].body' > comments.txt
      fi
      [[ "$has_label" == "true" ]] && gh pr edit "$number" --repo "${GITHUB_REPOSITORY}" --remove-label "needs-author-action"
      [[ "$has_ready_label" == "false" ]] && gh pr edit "$number" --repo "${GITHUB_REPOSITORY}" --add-label "ready-for-review"

      if [[ "$infra_finding" != "[]" ]]; then
        # Marker hashes the message, so a different red check later gets its own comment.
        if ! grep -qF -- "$infra_marker" comments.txt; then
          gh pr comment "$number" --repo "${GITHUB_REPOSITORY}" --body "$infra_marker
Everything else here looks ready, so this is labelled \`ready-for-review\`. ${infra_message}"
        fi
      fi
    fi
    continue
  fi

  ids=$(jq -r '[.[].id] | sort | join(",")' <<< "$blocking")
  marker="<!-- pr-readiness-check: $ids -->"
  # REST, fully paginated — a GraphQL comments(last: 100) cap could miss the marker on a busy PR.
  # Fetched to a file first, then searched — piping straight into `grep -q` would let grep close the
  # pipe on an early match, SIGPIPE-ing gh mid-fetch and (with pipefail) reporting failure even though
  # the marker was genuinely found.
  gh api "repos/${GITHUB_REPOSITORY}/issues/$number/comments" --paginate --jq '.[].body' > comments.txt
  already_reported=false
  grep -qF -- "$marker" comments.txt && already_reported=true

  [[ "$has_label" == "false" ]] && gh pr edit "$number" --repo "${GITHUB_REPOSITORY}" --add-label "needs-author-action"
  [[ "$has_ready_label" == "true" ]] && gh pr edit "$number" --repo "${GITHUB_REPOSITORY}" --remove-label "ready-for-review"
  [[ "$already_reported" == "true" ]] && continue

  items=$(jq -r '.[] | "- " + .message' <<< "$blocking")
  body=$(printf '%s\n\n@%s this PR needs a bit more before it can be reviewed:\n%s' "$marker" "$author" "$items")

  gh pr comment "$number" --repo "${GITHUB_REPOSITORY}" --body "$body"
done
