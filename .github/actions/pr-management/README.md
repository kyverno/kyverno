# Shared PR management

Reusable composite action for Kyverno repositories. Runs on `ubuntu-latest` with
Bash, Python 3.9+, Ruby (standard-library YAML/JSON), `jq`, and `gh`; no npm/pip
packages or repository-local tool actions are needed. Scripts are loaded from
`GITHUB_ACTION_PATH`, not the consuming checkout.

## Interface

| Input | Default | Meaning |
| --- | --- | --- |
| `mode` | required | `labels`, `readiness`, `stale`, `sync-labels`, or `validate` |
| `config` | `.github/pr-management.json` | Repository-specific JSON |
| `readiness-checks` | `.github/pr-readiness-checks.yml` | Repository-specific jq checks |
| `dry-run` | `'true'` | Exactly `'true'` or `'false'`; preview never mutates GitHub |
| `stale-days` | `'7'` | Author inactivity before marking stale |
| `review-nudge-days` | `'7'` | Maintainer inactivity before a reviewer reminder |

Supply `GH_TOKEN` in the caller step's environment, except for `validate`, which
requires no token or API access. The event and PR number come from GitHub's event
file. `readiness` handles `pull_request_target` opened/synchronize/reopened/
ready_for_review/converted_to_draft, hourly schedule, and workflow_dispatch.
`stale` normally runs daily/on demand; a draft transition in either mode clears
`ready-for-review`, `needs-author-action`, and `workflow-approval-required`.
Drafts and Dependabot are otherwise excluded from readiness/stale processing.

`labels` renders the metadata+rules label catalog for pinned `actions/labeler`.
On PR events, production mode applies path labels; on default-branch pushes it
refreshes merge-conflict labels. Preview prints the rendered rules, **not** an
exact matching simulation, and skips both external mutating actions.
`sync-labels` creates missing labels and updates changed colors/descriptions.
It never deletes unmanaged labels and fails on lookup or write errors.

`validate` checks JSON, label metadata/rules, and compiles all readiness jq
expressions without evaluating them, regardless of `dry-run`. It never calls
GitHub. Consumers can use it in read-only `pull_request` CI on local/head
configuration while pinning the action itself to a trusted full commit SHA.
When the action is pinned to a trusted SHA, the PR's configuration is only
parsed as YAML/JSON data; no PR-provided shell or Python is executed.

This repository's own `check-pr-management.yaml` intentionally runs the action
from the PR checkout, because it tests changes to the action itself. That is
ordinary `pull_request` CI: fork PRs receive a read-only token and no secrets,
and the job grants only `contents: read` with `persist-credentials: false`.

## Trusted configuration and permissions

For mutating or scheduled modes, **always check out the repository's default
branch**, never a PR head/merge ref, and set `persist-credentials: false`.
Readiness jq is executable data transformation, so only trusted default-branch
checks may be evaluated with a write-capable token. All configuration paths must
be files inside the checkout (including their resolved symlink destinations).
Do not run PR-provided code in these write-capable jobs.

```yaml
permissions:
  contents: read
  issues: write
  pull-requests: write
  checks: read
  actions: read

jobs:
  readiness:
    runs-on: ubuntu-latest
    timeout-minutes: 60
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: ${{ github.event.repository.default_branch }}
          persist-credentials: false
      # Replace <full-commit-SHA> with a reviewed engine commit before use.
      - uses: kyverno/kyverno/.github/actions/pr-management@<full-commit-SHA>
        env:
          GH_TOKEN: ${{ github.token }}
        with:
          mode: readiness
          dry-run: 'true'
```

Least-privilege scopes per mode:

| Mode | Required token permissions |
| --- | --- |
| `labels` | `contents: read`, `pull-requests: write` |
| `sync-labels` | `contents: read`, `issues: write` |
| `readiness` | `contents: read`, `issues: write`, `pull-requests: write`, `checks: read`, `actions: read` |
| `stale` | `contents: read`, `issues: write`, `pull-requests: write` |
| `validate` | `contents: read` (no token required) |

`labels` mode only applies path and `merge-conflicts` labels; it does not create
label metadata. Run `sync-labels` (for example on pushes to the default branch
that change `labels.yml`) so lifecycle labels such as `needs-author-action`,
`ready-for-review`, `workflow-approval-required`, and `stale` exist before
enabling readiness or stale mutations.
For `validate`, use only `contents: read`; checkout the proposed config and omit
`GH_TOKEN`. Use separate jobs for each mode, explicit timeouts, and non-cancelling
concurrency groups to avoid overlapping sweeps. Do not expose write-capable jobs
to `pull_request` or other untrusted-code execution.

## Repository configuration

All keys below are mandatory; unknown keys fail validation.

```json
{
  "fallback_reviewers": "@kyverno/website-maintainers",
  "exempt_labels": ["no-stale", "hold"],
  "close_days": 14,
  "warning_days": 7,
  "backlog_days": 30,
  "milestone_prefix": "",
  "labels_file": ".github/labels.yml"
}
```

Day values must be JSON integers from 1 through 3650; input day overrides must be
positive whole numbers in the same range. `stale-days` and `warning_days` must not
exceed `close_days`. Exemption labels are exact, case-sensitive names and apply
to **all** stale/idle-review handlers. `fallback_reviewers` is an
`@organization/team`. An empty `milestone_prefix` disables milestone stamping;
the engine uses `Kyverno Release ` and stamps qualifying same-repository issues
during readiness sweeps.

The label file maps each label name to `color` (quote six-digit hex values),
`description`, and optional actions/labeler `rules`. Rules are rendered without
changing matcher semantics. Metadata-only entries are not path-label rules.

Readiness checks map identifiers to jq `test` and `message` strings evaluated
against the fetched PR, with `$author` bound to its login. The optional
`infra_checks` identifier is advisory, not blocking. **Which checks count as
infrastructure is repository-specific**: maintain exclusions and explanations in
the caller's YAML, never in the generic scripts. Engine infra exclusions remain
in `.github/pr-readiness-checks.yml`.

## Safety and rollout

Readiness finishes label/check/thread pagination before evaluation. Missing CI
data and pending/gated workflows are not ready; failed lookups and partial data
never prove readiness. A remaining truncated connection may remove a stale
ready label but never clears author-action/approval labels. The head/state/draft
are rechecked before readiness changes. Comments are paginated for marker
deduplication.

Stale handling preserves commit/force-push/merge-conflict clocks, conservative
observation of backdated pushes, and bot-only persisted markers. Author replies
do not reset inactivity. Auto-close requires both total `close_days` inactivity
**and** at least `warning_days` since a matching bot warning or bot-applied stale
label. A manually applied stale label without a bot warning is insufficient.
An old backlog PR cannot be marked and closed in the same sweep. State, draft
status, and exemptions are rechecked from fresh data; close additionally checks
live state/head/labels immediately before writing. API races cannot be made
transactional, but changed/unknown data fails conservatively.

Start new repositories with readiness/stale/sync in preview. Inspect scheduled
logs and reviewer mentions, provision label metadata, then enable each mode
separately by explicitly setting `dry-run: 'false'`. Keep manual dispatch preview
as the default. Engine automation explicitly uses production mode to preserve
its running behavior. Labels can be enabled independently to preserve existing
path labeling while other modes are previewed.

## Regression tests

From the repository root:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s .github/actions/pr-management/tests -v
PR_MODE=validate python3 .github/actions/pr-management/run.py
for script in .github/actions/pr-management/*.sh; do bash -n "$script"; done
actionlint -shellcheck='' .github/workflows/{pr-labelling,pr-readiness-check,pr-stale-author-action,sync-label-colors,check-pr-management}.yaml
```

Tests use Python's standard library and a fixture `gh` executable, run the
extracted programs/real entry point, and never contact GitHub. Scratch files stay
inside the project and are removed. CI runs tests, offline config validation,
and shell syntax checks when shared automation/config/callers change.
