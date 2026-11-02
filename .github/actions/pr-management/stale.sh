#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

[[ "$STALE_DAYS" =~ ^[0-9]+$ && "$CLOSE_DAYS" =~ ^[0-9]+$ && "$NUDGE_DAYS" =~ ^[0-9]+$ ]] \
  || { echo "::error::configured day values must be whole numbers."; exit 1; }
(( STALE_DAYS <= CLOSE_DAYS )) \
  || { echo "::error::stale_days must not exceed close_days."; exit 1; }

repo="${GITHUB_REPOSITORY}"
now=$(date -u +%s)
stale_secs=$((STALE_DAYS * 86400))
close_secs=$((CLOSE_DAYS * 86400))
warning_secs=$((WARNING_DAYS * 86400))
nudge_secs=$((NUDGE_DAYS * 86400))
backlog_secs=$((BACKLOG_DAYS * 86400))

# Mutating gh calls go through here so a dry run only prints them.
act() {
  if [[ "$DRY_RUN" == "true" ]]; then echo "[dry-run] $*"; else "$@" && sleep 1; fi
}

# PR data is arbitrary text, so normalise it into small files rather than bash variables.
# Helpers return non-zero on failure.
pr_query='
  query($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) {
      pullRequest(number: $number) {
        headRefOid
        state
        isDraft
        author { login }
        labels(first: 100) { pageInfo { hasNextPage } nodes { name } }
        commits(last: 100) { nodes { commit { oid committedDate } } }
        assignees(first: 20) { nodes { login } }
        reviewRequests(first: 50) { nodes { requestedReviewer {
          __typename
          ... on User { login }
          ... on Bot { login }
          ... on Mannequin { login }
          ... on Team { combinedSlug }
        } } }
        comments(last: 100) {
          totalCount
          pageInfo { hasPreviousPage startCursor }
          nodes { author { login __typename } createdAt body }
        }
        reviews(last: 100) { totalCount nodes { author { login __typename } submittedAt } }
        timelineItems(last: 100, itemTypes: [LABELED_EVENT, UNLABELED_EVENT, HEAD_REF_FORCE_PUSHED_EVENT, PULL_REQUEST_COMMIT]) {
          pageInfo { hasPreviousPage startCursor }
          nodes {
            __typename
            ... on LabeledEvent { createdAt actor { __typename } label { name } }
            ... on UnlabeledEvent { createdAt label { name } }
            ... on HeadRefForcePushedEvent { createdAt }
            ... on PullRequestCommit { commit { oid committedDate } }
          }
        }
      }
    }
  }'
fetch() {
  local try
  for try in 1 2 3; do
    gh api graphql -f query="$pr_query" -f owner="${repo%/*}" -f name="${repo#*/}" -F number="$1" > raw.json && break
    [[ "$try" -eq 3 ]] && return 1
    sleep 2
  done
  jq -e '.data.repository.pullRequest |
    .state == "OPEN" and .isDraft == false and .author.login != "dependabot[bot]" and
    (.headRefOid | type == "string" and test("^[0-9a-f]{40,64}$")) and
    (.labels.nodes | type == "array") and .labels.pageInfo.hasNextPage == false and
    (.commits.nodes | type == "array" and length > 0) and
    (.comments.nodes | type == "array") and
    (.comments.pageInfo.hasPreviousPage | type == "boolean") and
    (.reviews.nodes | type == "array") and
    (.timelineItems.nodes | type == "array") and
    (.timelineItems.pageInfo.hasPreviousPage | type == "boolean")' raw.json > /dev/null || return 1
  jq '.data.repository.pullRequest
    | {author: {login: (.author.login // "ghost")}, headRefOid,
       labels: .labels.nodes,
       commits: [.commits.nodes[].commit | {oid, committedDate}],
       assignees: .assignees.nodes,
       reviewRequests: [.reviewRequests.nodes[].requestedReviewer
         | select(. != null and (.login or .combinedSlug))
         | if .__typename == "Team" then {__typename, slug: .combinedSlug} else {__typename, login} end]}' \
    raw.json > pr.json || return 1
  jq -n '[]' > comment-pages.json || return 1
  jq '.data.repository.pullRequest.comments.pageInfo' raw.json > comment-page-info.json || return 1
  append_comment_page raw.json || return 1
  # Markers can be older than the newest 100 comments. Complete the history before any decision.
  local comment_page=0 previous_cursor="" cursor
  while [[ "$(jq -r '.hasPreviousPage' comment-page-info.json)" == "true" ]]; do
    comment_page=$((comment_page + 1))
    [[ "$comment_page" -le 100 ]] || return 1
    cursor=$(jq -r '.startCursor' comment-page-info.json)
    [[ -n "$cursor" && "$cursor" != "null" && "$cursor" != "$previous_cursor" ]] || return 1
    previous_cursor="$cursor"
    gh api graphql -f query="$comments_query" -f owner="${repo%/*}" -f name="${repo#*/}" \
      -F number="$1" -f cursor="$cursor" > comments-page.json || return 1
    jq -e '.data.repository.pullRequest.comments |
      (.nodes | type == "array") and (.pageInfo.hasPreviousPage | type == "boolean")' \
      comments-page.json > /dev/null || return 1
    jq '.data.repository.pullRequest.comments.pageInfo' comments-page.json > comment-page-info.json || return 1
    append_comment_page comments-page.json || return 1
  done
  jq '[.data.repository.pullRequest.reviews.nodes[] | select(.submittedAt != null)
       | {u: (.author.login // "ghost"), t: (.author.__typename // "User"), at: .submittedAt}]' \
    raw.json > reviews.json || return 1
  jq -n '[]' > timeline-pages.json || return 1
  jq '.data.repository.pullRequest.timelineItems.pageInfo' raw.json > timeline-page-info.json || return 1
  append_timeline_page raw.json || return 1
  local timeline_page=0 previous_timeline_cursor=""
  while needs_timeline_history; do
    timeline_page=$((timeline_page + 1))
    [[ "$timeline_page" -le 100 ]] || return 1
    local cursor
    cursor=$(jq -r '.startCursor' timeline-page-info.json)
    [[ -n "$cursor" && "$cursor" != "null" && "$cursor" != "$previous_timeline_cursor" ]] || return 1
    previous_timeline_cursor="$cursor"
    gh api graphql -f query="$timeline_query" -f owner="${repo%/*}" -f name="${repo#*/}" \
      -F number="$1" -f cursor="$cursor" > timeline-page.json || return 1
      jq -e '.data.repository.pullRequest.timelineItems |
        (.nodes | type == "array") and (.pageInfo.hasPreviousPage | type == "boolean")' \
        timeline-page.json > /dev/null || return 1
    jq '.data.repository.pullRequest.timelineItems.pageInfo' timeline-page.json > timeline-page-info.json || return 1
    append_timeline_page timeline-page.json || return 1
  done
  jq '[.[] | select(.kind == "label") | {l, at, t}]' timeline-events.json > label-events.json || return 1
  jq '[.[] | select(.kind == "unlabel") | {l, at}]' timeline-events.json > unlabel-events.json || return 1
  jq '[.[] | select(.kind == "force-push") | {at}]' timeline-events.json > force-push-events.json || return 1
}

timeline_query='
  query($owner: String!, $name: String!, $number: Int!, $cursor: String!) {
    repository(owner: $owner, name: $name) {
      pullRequest(number: $number) {
        timelineItems(last: 100, before: $cursor, itemTypes: [LABELED_EVENT, UNLABELED_EVENT, HEAD_REF_FORCE_PUSHED_EVENT, PULL_REQUEST_COMMIT]) {
          pageInfo { hasPreviousPage startCursor }
          nodes {
            __typename
            ... on LabeledEvent { createdAt actor { __typename } label { name } }
            ... on UnlabeledEvent { createdAt label { name } }
            ... on HeadRefForcePushedEvent { createdAt }
            ... on PullRequestCommit { commit { oid committedDate } }
          }
        }
      }
    }
  }'

comments_query='
  query($owner: String!, $name: String!, $number: Int!, $cursor: String!) {
    repository(owner: $owner, name: $name) {
      pullRequest(number: $number) {
        comments(last: 100, before: $cursor) {
          pageInfo { hasPreviousPage startCursor }
          nodes { author { login __typename } createdAt body }
        }
      }
    }
  }'

append_comment_page() {
  local page="$1"
  jq '[.data.repository.pullRequest.comments.nodes[]
       | {u: (.author.login // "ghost"), t: (.author.__typename // "User"), at: .createdAt, body: (.body // "")}]' \
    "$page" > comment-current-page.json || return 1
  jq --slurpfile page comment-current-page.json '. + [$page[0]]' \
    comment-pages.json > comment-pages-next.json || return 1
  mv comment-pages-next.json comment-pages.json
  jq 'reverse | add // []' comment-pages.json > issue-comments.json || return 1
}

append_timeline_page() {
  local page="$1"
  jq '[.data.repository.pullRequest.timelineItems.nodes[]
       | if .__typename == "LabeledEvent" then {kind: "label", l: .label.name, at: .createdAt, t: .actor.__typename}
         elif .__typename == "UnlabeledEvent" then {kind: "unlabel", l: .label.name, at: .createdAt}
         elif .__typename == "HeadRefForcePushedEvent" then {kind: "force-push", at: .createdAt}
         elif .__typename == "PullRequestCommit" then {kind: "commit", oid: .commit.oid, at: .commit.committedDate}
         else empty end]' "$page" > timeline-current-page.json || return 1
  jq --slurpfile page timeline-current-page.json '. + [$page[0]]' \
    timeline-pages.json > timeline-pages-next.json || return 1
  mv timeline-pages-next.json timeline-pages.json
  jq 'reverse | add // []' timeline-pages.json > timeline-events.json || return 1
}

needs_timeline_history() {
  [[ "$(jq -r '.hasPreviousPage' timeline-page-info.json)" == "true" ]] || return 1
  local labels
  labels=$(jq -r '[.labels[].name] | join(",")' pr.json)
  for label in needs-author-action stale ready-for-review merge-conflicts; do
    if [[ ",$labels," == *",$label,"* ]] \
      && ! jq -e --arg l "$label" 'any(.[]; .kind == "label" and .l == $l)' timeline-events.json > /dev/null; then
      return 0
    fi
  done
  if [[ ",$labels," == *",stale,"* ]] \
    && [[ "$(jq -r '.headRefOid' pr.json)" != "null" ]] \
    && ! has_marker 'pr-stale-author-action: stale ' \
    && ! jq -e '
      ([to_entries[] | select(.value.kind == "label" and .value.l == "stale") | .key] | last // -1) as $stale
      | any(to_entries[]; .key < $stale and .value.kind == "commit")
    ' timeline-events.json > /dev/null; then
    return 0
  fi
  return 1
}

has_label() { jq -e --arg l "$1" 'any(.labels[]; .name == $l)' pr.json > /dev/null; }
is_exempt() {
  jq -e --argjson exemptions "$EXEMPT_LABELS" \
    'any(.labels[]; .name as $l | $exemptions | index($l) != null)' pr.json > /dev/null
}
# Only a bot's comment counts as a marker, so an author can't plant one to suppress a reminder.
has_marker() { jq -e --arg m "$1" 'any(.[]; .t == "Bot" and (.body | contains($m)))' issue-comments.json > /dev/null; }

# Epoch of the most recent time label $1 was added; 0 if never.
label_time() {
  jq --arg l "$1" '[.[] | select(.l == $l) | .at | fromdateiso8601] | max // 0' label-events.json
}

# A manual stale label alone is not evidence that the contributor received a bot warning.
warning_time() {
  local marker="$1" stale_at="$2"
  jq -n --arg marker "$marker" --argjson stale "$stale_at" \
    --slurpfile comments issue-comments.json --slurpfile labels label-events.json '
    [($comments[0][] | select(.t == "Bot" and (.body | contains($marker)))
       | .at | fromdateiso8601 | select(. >= $stale)),
     ($labels[0][] | select(.l == "stale" and .t == "Bot") | .at | fromdateiso8601
       | select(. >= $stale))]
    | max // 0'
}

# Epoch of the latest commit, from anyone: a maintainer or a second account pushing is activity too.
last_commit() {
  jq '[.commits[].committedDate | fromdateiso8601] | max // 0' pr.json
}

latest_force_push() {
  jq '[.[] | .at | fromdateiso8601] | max // 0' force-push-events.json
}

latest_stale_removal() {
  jq '[.[] | select(.l == "stale") | .at | fromdateiso8601] | max // 0' unlabel-events.json
}

commit_after_label() {
  jq -e --arg l "$1" '
    ([to_entries[] | select(.value.kind == "label" and .value.l == $l) | .key] | last // -1) as $label_index
    | any(to_entries[]; .key > $label_index and .value.kind == "commit")
  ' timeline-events.json > /dev/null
}

late_commit_oid() {
  jq -r '
    ([to_entries[] | select(.value.kind == "label" and .value.l == "needs-author-action") | .key] | last // -1) as $label_index
    | [to_entries[] | select(.key > $label_index and .value.kind == "commit") | .value.oid]
    | last // ""
  ' timeline-events.json
}

# GitHub does not expose ordinary push times; persist when this workflow first sees a commit added after the label.
late_commit_observed_at() {
  local n="$1" oid observed cursor
  oid=$(late_commit_oid) || return 1
  [[ -z "$oid" ]] && { printf '0'; return 0; }
  observed=$(jq -r --arg oid "$oid" '
    [.[] | select(.t == "Bot") | .body
     | try capture("<!-- pr-stale-author-action: observed (?<oid>[0-9a-f]+) (?<at>[0-9]+) -->") catch empty
     | select(.oid == $oid) | .at] | last // ""
  ' issue-comments.json) || return 1
  while [[ -z "$observed" && "$(jq -r '.hasPreviousPage' comment-page-info.json)" == "true" ]]; do
    cursor=$(jq -r '.startCursor' comment-page-info.json)
    gh api graphql -f query="$comments_query" -f owner="${repo%/*}" -f name="${repo#*/}" \
      -F number="$n" -f cursor="$cursor" > comments-page.json || return 1
    jq '.data.repository.pullRequest.comments.pageInfo' comments-page.json > comment-page-info.json || return 1
    append_comment_page comments-page.json || return 1
    observed=$(jq -r --arg oid "$oid" '
      [.[] | select(.t == "Bot") | .body
       | try capture("<!-- pr-stale-author-action: observed (?<oid>[0-9a-f]+) (?<at>[0-9]+) -->") catch empty
       | select(.oid == $oid) | .at] | last // ""
    ' issue-comments.json) || return 1
  done
  if [[ -n "$observed" ]]; then
    printf '%s' "$observed"
    return 0
  fi
  local marker="<!-- pr-stale-author-action: observed $oid $now -->"
  printf '%s\n' "$marker" > body.md
  act gh pr comment "$n" --repo "$repo" --body-file body.md >&2 || return 1
  printf '%s' "$now"
}

force_push_after() {
  jq -e --argjson t "$1" 'any(.[]; (.at | fromdateiso8601) > $t)' force-push-events.json > /dev/null
}

stale_head() {
  local marked
  marked=$(jq -r '[.[] | select(.t == "Bot") | .body
    | try capture("<!-- pr-stale-author-action: stale [0-9]+ (?<head>[0-9a-f]+|null) -->").head catch empty]
    | map(select(. != "")) | last // ""' issue-comments.json) || return 1
  if [[ -n "$marked" ]]; then
    printf '%s' "$marked"
    return 0
  fi
  if [[ "$(jq -r '.headRefOid' pr.json)" == "null" ]]; then
    printf 'null'
    return 0
  fi
  jq -r '
    ([to_entries[] | select(.value.kind == "label" and .value.l == "stale") | .key] | last // -1) as $stale
    | if $stale < 0 then ""
      else ([to_entries[] | select(.key < $stale and .value.kind == "commit") | .value.oid] | last // "")
      end' timeline-events.json
}

# Use commit/force-push activity and stale-label removal; undated late pushes start a conservative clock.
author_clock() {
  local n="$1" last pushed removed conflicted observed
  last=$(last_commit) || return 1
  pushed=$(latest_force_push) || return 1
  [[ "$pushed" -gt "$last" ]] && last="$pushed"
  removed=$(latest_stale_removal) || return 1
  [[ "$removed" -gt "$last" ]] && last="$removed"
  if has_label merge-conflicts; then
    conflicted=$(label_time merge-conflicts) || return 1
    [[ "$conflicted" -gt "$last" ]] && last="$conflicted"
  fi
  observed=$(late_commit_observed_at "$n") || return 1
  [[ "$observed" -gt "$last" ]] && last="$observed"
  printf '%s' "$last"
}

# Epoch the PR started waiting on reviewers: newest of label time ($2), the last commit (anyone's, since a
# new push needs a fresh review), or the last non-author, non-bot comment/review.
maintainer_clock() {
  jq -n --arg a "$1" --argjson ready "$2" \
    --slurpfile pr pr.json --slurpfile ic issue-comments.json --slurpfile rv reviews.json '
    [ $ready,
      ($pr[0].commits[].committedDate | fromdateiso8601),
      (($ic[0][], $rv[0][]) | select(.u != $a and .t != "Bot") | .at | fromdateiso8601) ]
    | max'
}

reviewers() {
  jq -r -n --arg a "$1" --arg fb "$FALLBACK_REVIEWERS" \
    --slurpfile pr pr.json --slurpfile rv reviews.json '
    [ ($pr[0].reviewRequests[] | if .__typename == "Team" then "@\(.slug)" else "@\(.login)" end),
      ($pr[0].assignees[] | "@\(.login)"),
      ($rv[0][] | select(.t != "Bot") | "@\(.u)") ]
    | map(select(. != "@" + $a and (test("\\[bot\\]$") | not) and . != "@Copilot"))
    | unique
    | if length == 0 then $fb else join(" ") end'
}

date_of() { jq -rn --argjson t "$1" '$t | strftime("%Y-%m-%d")'; }

# PRs carrying `stale`: clear it once it no longer applies. Only a new commit counts: a reply (for example
# "I can't continue") must not clear it.
handle_stale_label() {
  local n="$1" last stale_at head recorded_head
  fetch "$n" || return 1
  # Exemption resets the cycle: otherwise removing the exemption later could close on the old stale clock.
  if is_exempt; then
    has_label stale || return 0
    echo "PR #$n: exempt — removing the stale label."
    act gh pr edit "$n" --repo "$repo" --remove-label stale || return 1
    return 0
  fi
  last=$(last_commit) || return 1
  stale_at=$(label_time stale) || return 1
  head=$(jq -r '.headRefOid' pr.json)
  recorded_head=$(stale_head) || return 1
  # Unknown when the label event is outside the fetched history; leave the label alone.
  [[ "$stale_at" -eq 0 ]] && { echo "::warning::PR #$n: stale label time not found — skipping."; return 0; }
  if ! has_label needs-author-action || [[ "$last" -gt "$stale_at" ]] \
    || commit_after_label stale \
    || force_push_after "$stale_at" \
    || { [[ "$head" != "null" && -n "$recorded_head" ]] && [[ "$head" != "$recorded_head" ]]; }; then
    echo "PR #$n: no longer stale — removing the label."
    act gh pr edit "$n" --repo "$repo" --remove-label stale || return 1
  fi
}

# needs-author-action PRs: mark stale after STALE_DAYS without a new commit.
handle_needs_author() {
  local n="$1" author last idle marker head needs_author_at
  fetch "$n" || return 1
  is_exempt && return 0
  has_label needs-author-action || return 0
  has_label stale && return 0
  author=$(jq -r '.author.login' pr.json)
  needs_author_at=$(label_time needs-author-action) || return 1
  [[ "$needs_author_at" -eq 0 ]] && { echo "::warning::PR #$n: needs-author-action label time not found — skipping."; return 0; }
  last=$(author_clock "$n") || return 1
  head=$(jq -r '.headRefOid' pr.json)
  idle=$((now - last))
  [[ "$idle" -lt "$stale_secs" ]] && return 0

  marker="<!-- pr-stale-author-action: stale $last $head -->"
  if ! has_marker "$marker"; then
    {
      printf '%s\n' "$marker"
      printf '@%s there has been no new commit on this PR for %s days (since: %s), and it is still waiting on you (`needs-author-action`), so it is now marked `stale`.\n\n' \
        "$author" "$((idle / 86400))" "$(date_of "$last")"
      printf -- '- Still working on it? Push a commit; the label clears automatically.\n'
      printf -- '- Stuck or unsure? Say what you are stuck on and a reviewer will help.\n'
      printf -- '- Cannot continue? Say so here; the label stays and a maintainer can pick it up.\n\n'
      # Old PRs: author only, no reviewer ping.
      if [[ "$idle" -lt "$backlog_secs" ]]; then
        printf '%s no new commit for %s days; this may need a nudge, a hand-off, or closing.' \
          "$(reviewers "$author")" "$((idle / 86400))"
        if has_label milestone-pr; then
          printf '\n\nThis PR closes an issue in an open release milestone, so maintainers: consider taking it over or reassigning it.'
        fi
      fi
    } > body.md
    [[ "$DRY_RUN" == "true" ]] && { echo "PR #$n would get:"; cat body.md; echo; }
    act gh pr comment "$n" --repo "$repo" --body-file body.md || return 1
  fi
  echo "PR #$n: no new commit for $((idle / 86400)) days — marking stale."
  act gh pr edit "$n" --repo "$repo" --add-label stale
}

# Close a PR after CLOSE_DAYS total inactivity, once it has already been marked stale.
handle_close() {
  local n="$1" author last last_commit_at stale_at idle marker head recorded_head warned_at
  fetch "$n" || return 1
  if ! has_label needs-author-action || ! has_label stale || is_exempt; then
    return 0
  fi
  author=$(jq -r '.author.login' pr.json)
  last_commit_at=$(last_commit) || return 1
  last=$(author_clock "$n") || return 1
  stale_at=$(label_time stale) || return 1
  head=$(jq -r '.headRefOid' pr.json)
  recorded_head=$(stale_head) || return 1
  # A new commit or force-push after stale marking resets the cycle; merge-conflict time only affects idle.
  [[ "$stale_at" -eq 0 || "$last_commit_at" -gt "$stale_at" ]] && return 0
  commit_after_label stale && return 0
  force_push_after "$stale_at" && return 0
  [[ -z "$recorded_head" ]] && return 0
  [[ "$head" != "null" && "$head" != "$recorded_head" ]] && return 0
  idle=$((now - last))
  [[ "$idle" -lt "$close_secs" ]] && return 0
  warned_at=$(warning_time "<!-- pr-stale-author-action: stale $last $head -->" "$stale_at") || return 1
  [[ "$warned_at" -gt 0 && $((now - warned_at)) -ge "$warning_secs" ]] || return 0
  # Recheck head/state/exemptions immediately before a destructive action.
  gh api "repos/$repo/pulls/$n" > live-pr.json || return 1
  jq -e --arg head "$head" --argjson exemptions "$EXEMPT_LABELS" '
    .state == "open" and .draft == false and .head.sha == $head and
    ([.labels[].name] | index("stale") != null and index("needs-author-action") != null) and
    (all(.labels[]; .name as $l | $exemptions | index($l) == null))
  ' live-pr.json > /dev/null || return 1

  marker="<!-- pr-stale-author-action: close-$last -->"
  if ! has_marker "$marker"; then
    {
      printf '%s\n' "$marker"
      printf '@%s to help manage our PR queue we are closing PRs with no activity. This PR has had no new commit for %s days. A prior reminder was issued when this PR was marked `stale`. If you would like to continue working on it, please address the flagged issues and reopen the PR. Thanks for your contributions and understanding!\n' \
        "$author" "$((idle / 86400))"
    } > body.md
    [[ "$DRY_RUN" == "true" ]] && { echo "PR #$n would get:"; cat body.md; echo; }
    act gh pr comment "$n" --repo "$repo" --body-file body.md || return 1
  fi
  echo "PR #$n: no new commit for $((idle / 86400)) days — closing."
  act gh pr close "$n" --repo "$repo"
}

# ready-for-review PRs: nudge the reviewers once per head commit after NUDGE_DAYS of maintainer silence.
handle_ready() {
  local n="$1" author ready_at since marker head
  fetch "$n" || return 1
  is_exempt && return 0
  has_label ready-for-review || return 0
  author=$(jq -r '.author.login' pr.json)
  ready_at=$(label_time ready-for-review) || return 1
  [[ "$ready_at" -eq 0 ]] && { echo "::warning::PR #$n: ready-for-review label time not found — skipping."; return 0; }
  since=$(maintainer_clock "$author" "$ready_at") || return 1
  [[ $((now - since)) -lt "$nudge_secs" ]] && return 0
  # Old PRs: no reviewer ping.
  [[ $((now - since)) -ge "$backlog_secs" ]] && { echo "PR #$n: idle for $(((now - since) / 86400)) days — skipping the nudge."; return 0; }

  head=$(jq -r '.headRefOid' pr.json)
  marker="<!-- pr-review-nudge: $head -->"
  has_marker "$marker" && return 0

  {
    printf '%s\n' "$marker"
    printf '%s this PR has been `ready-for-review` with no maintainer activity for %s days (since %s). Could someone take a look?\n\n' \
      "$(reviewers "$author")" "$(((now - since) / 86400))" "$(date_of "$since")"
    printf '@%s nothing is needed from you; this is just a reminder to the reviewers.' "$author"
  } > body.md
  echo "PR #$n: ready-for-review with no maintainer activity for $(((now - since) / 86400)) days — nudging."
  [[ "$DRY_RUN" == "true" ]] && { cat body.md; echo; }
  act gh pr comment "$n" --repo "$repo" --body-file body.md
}

list() { gh pr list --repo "$repo" --state open --limit 1000 --search "$1 sort:created-asc" --json number --jq '.[].number'; }
failed=0
run_pr() {
  if ! "$1" "$2"; then echo "::error::PR #$2: $1 failed — skipping."; failed=$((failed + 1)); fi
}
# `for n in $(list ...)` would swallow a failed listing, so list first and count the failure.
sweep() {
  local prs n
  if ! prs=$(list "$1"); then echo "::error::Listing PRs for '$1' failed."; failed=$((failed + 1)); return 0; fi
  [[ $(wc -w <<< "$prs") -ge 1000 ]] && echo "::warning::'$1' matched 1000 PRs; any beyond that are not examined."
  for n in $prs; do run_pr "$2" "$n"; done
}

if [[ "${PR_MANAGEMENT_LIBRARY_ONLY:-false}" == "true" ]]; then return 0; fi

base="draft:false -author:app/dependabot"
sweep "$base label:stale" handle_stale_label
sweep "$base label:needs-author-action -label:stale" handle_needs_author
sweep "$base label:needs-author-action label:stale" handle_close
sweep "$base label:ready-for-review" handle_ready

if [[ "$failed" -gt 0 ]]; then echo "::error::$failed operation(s) failed."; exit 1; fi
