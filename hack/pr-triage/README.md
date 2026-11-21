# Legacy PR migration tooling

This is maintainer-operated tooling for moving reviewed legacy-policy changes
from `main` to `release-1.19`. Preparing or previewing a migration never writes
to GitHub. Publishing is a separate, explicitly approved operation, not part of
the October 20 dry run.

Requirements: Python 3.9+, Git, authenticated `gh`, and `jq` for `apply-labels.sh`.
Run from a Kyverno clone. Keep artifacts outside the checkout. Do not run
validation commands from an untrusted PR with a write-capable token in its
environment.

## 1. Prepare a new manifest

```sh
python3 -B hack/pr-triage/pr-branch-triage.py \
  --repo kyverno/kyverno --base main --skip-bots --diff-scan --limit 1000 \
  --out /tmp/legacy-triage.md --json /tmp/legacy-triage.json

python3 -B hack/pr-triage/prepare-retarget.py \
  --report /tmp/legacy-triage.json --output /tmp/legacy-migration --workers 3
```

Use a **new** output directory for each run. Do not use `--reclassify` with an
old snapshot to approve a live migration. Check that the PR inventory did not
hit `--limit` before approving it.

Preparation pins the current `main` and `release-1.19` SHAs, checks captured PR
state/head/base/metadata and the complete changed-file inventory, and probes
every legacy candidate in an isolated worktree. Hooks, automatic ref updates,
and rerere are disabled for these probes. It never runs PR code or tests.
Before checkout it recomputes path, content and migration routing with the
current classifier against the pinned original diff; stale report signals block
preparation rather than authorizing a legacy recommendation.
Checkout/rebase runs in a freshly initialized temporary repository, with system
and global Git configuration disabled and inherited `GIT_*` variables removed.
It does not inherit the operator clone's local filters, merge drivers or templates.

The output contains:

| Artifact | Purpose |
|---|---|
| `manifest.json` | Per-PR identity, probe result, recommendation and initially null approval |
| `manifest.md` | Human-readable decision table |
| `review.json` | The same decisions without PR bodies and redundant classifier fields; suitable for committing as a dated review snapshot |
| `<PR>/original.diff`, `shared.diff`, `commits.txt` | Original scope and shared-file hunks |
| `<PR>/final.diff`, `range-diff.txt` | Successful rebase evidence |
| `<PR>/rebase.log`, `candidate.json` | Conflict/error details and per-PR checkpoint |

Original and clean rebased commits remain under unique
`refs/retarget/prepared/<run>/<PR>/{original,rebased}` refs. They survive
worktree cleanup and must be retained, along with the artifact directory and
journal, until the migration/rollback window closes. They are local to this
clone; a committed review snapshot does not recreate them in another clone.
Each evidence file is bound to the candidate by `evidence_sha256`; publication
verifies its bytes and requires the approval to repeat the complete digest map.
Preparation failures are explicit holds, never publishable fallback results.

`RETARGET` is a **recommendation**, not authorization. Drafts, features, shared
hunks, overlaps with open release PRs or other candidates, and conflicts/errors
are held as `NEEDS_AUTHOR`. Path overlap is a review lead, not proof of duplicate
fixes. `EMPTY` means no commits survived rebase; inspect patch equivalence before
deciding whether the original can be closed. The tooling never closes originals.

Review `shared.diff` before deciding whether a shared edit belongs on `main`,
needs a split, or is a necessary dependency of a legacy-only fix. Keep
migration/removal work on `main`. Features require a release-maintainer decision.
CEL fixes are legitimate backports to 1.19; CEL classification is not a blanket
release-branch prohibition.

## 2. Review and approve one candidate

Compare the original and rebased commit ranges and the complete final diff.
Check for already-landed fixes and overlapping open PRs. Resolve conflicts in
an author/maintainer-owned branch and **prepare a fresh manifest** afterward;
the publisher does not resolve conflicts.

Run appropriate validation on the exact prepared SHA in a separate, unprivileged
environment. Use the release branch's documented test commands: relevant Go
unit tests, CLI fixtures for observable CLI behavior, codegen for generated/API
changes, and targeted Chainsaw conformance for runtime behavior. A clean rebase
is not a passing test. Account for known baseline failures independently.

Only after this review, change a held recommendation to `RETARGET` if justified
and fill the candidate's approval:

```json
{
  "reviewer": "maintainer-login",
  "head_sha": "<captured original SHA>",
  "target_sha": "<captured release-1.19 SHA>",
  "rebased_sha": "<reviewed prepared SHA>",
  "evidence_sha256": {"<each evidence key>": "<captured SHA-256 digest>"},
  "range_diff_reviewed": true,
  "final_diff_reviewed": true,
  "tests": [{"command": "<exact validation command>", "result": "passed"}],
  "release_feature_approved": false
}
```

The approval records a human decision; it is not a substitute for GitHub branch
protection or renewed reviewer approval after publication. The reviewer must be
different from the PR author. Execution verifies the recorded reviewer has live upstream write,
maintain or admin permission. This local operator-owned approval record is not
a cryptographic signature or proof that the named reviewer authored it.
`release_feature_approved` is a required boolean and must be true for feature-titled PRs. All approvals in the
committed October snapshot are intentionally null. Approval of the tooling PR
does not approve any candidate.

## 3. Preview, then publish separately

```sh
python3 -B hack/pr-triage/retarget.py \
  --manifest /tmp/legacy-migration/manifest.json --pr 15432 \
  --journal /tmp/legacy-migration/15432-journal.json
```

The default is a preview, including blocked approval conditions, with no
GitHub or journal writes. Explicitly select **one PR** per invocation.
Before any execution, arrange a scoped maintenance window for the existing
`PR Branch Auto-Updater` and any other automation that can push to this PR.
Execution also rejects Git URL-rewrite rules (`insteadOf`/`pushInsteadOf`) in
any configuration scope, including included files. Remove these rules before
publishing; inherited `GIT_*` overrides are ignored. Normal file-configured
GitHub credential helpers remain available.
The publisher requires `--updater-paused` as an operator attestation; it does
not disable shared automation itself. Coordinate the pause and restoration
with maintainers rather than leaving the updater disabled indefinitely.

The default `strategy: "sibling"` preserves the author's branch and original
PR. Supply `--body-file` with a reviewed body based on the repository PR
template, linking the original PR. Publishing creates a uniquely named upstream
branch with a create-only lease and opens a PR against `release-1.19`.
The publisher appends a hidden run-identity comment to the reviewed body and
records its exact digest and authenticated actor in the journal. Reconciliation
requires these to match before adopting or closing a discovered sibling. A
foreign or edited PR is a manual-reconciliation hold, never silently adopted.
It does not copy approvals or close the original.

`strategy: "author"` additionally requires live `maintainerCanModify` and an
author-written consent comment whose first line is:

```text
/retarget release-1.19 <captured-original-head-sha>
```

Record its numeric `consent_comment_id` in the candidate. Consent is bound to the
exact head, not an old unqualified slash command or silence. The publisher
pushes the **prepared SHA**, with an explicit expected-SHA lease against the
actual fork repository/ref, then changes the PR base. It never uses the caller's
checkout `HEAD`. GitHub has no compare-and-swap base-edit API: immediately
rechecking state narrows but cannot eliminate the final base-edit race. Keep
the maintenance window and monitor the journal/live state.

**The following is an execution command, not part of the dry run:**

```sh
python3 -B hack/pr-triage/retarget.py \
  --manifest /tmp/legacy-migration/manifest.json --pr 15432 \
  --journal /tmp/legacy-migration/15432-journal.json \
  --body-file /tmp/reviewed-sibling-pr-body.md \
  --updater-paused --execute
```

Initial execution refuses changed source/target branches or stale PR identities.
Re-prepare and re-review when those checks fail; do not edit captured SHAs to
bypass them. Labeling and notice comments also change PR metadata, so perform
those separately and regenerate afterward.

## Resume and rollback

The journal is locked locally, atomically replaced and fsynced, with write-ahead
states before remote writes. Use the same manifest, PR selection, body and
journal with `--resume --execute` after a partial failure. Read-back of remote
heads/bases and existing sibling PRs prevents replaying a write blindly.
If an author base edit has an unknown outcome and the live base is still `main`,
resume refuses to repeat it: an earlier successful edit may have been undone by
another actor. Reconcile manually or roll back, then prepare a fresh migration.
The same no-replay rule applies to an unknown rollback base edit: if the base
has not reached `main`, reconcile manually instead of retrying the edit.
Sibling title/body inputs are bound when the journal is created, before the
first remote push; changing them requires rollback and a new reviewed journal.
Do not delete a journal to work around an error; retain it for reconciliation.
Local locking does not coordinate independent clones: the operator must enforce
one migration owner per PR.

Forward resume still checks the pinned source/target branches. If either moved
after a partial publication, reconcile with rollback (which does not require
unchanged branch tips), then prepare and approve a fresh manifest. Do not swap
the manifest underneath an existing journal.

Add `--rollback` without `--execute` to preview recovery, then explicitly approve
its execution. Author rollback restores the original SHA with a lease against
the migration's SHA and restores the original base. It refuses to overwrite new
author work. Sibling rollback closes only the journal-owned, unmerged sibling;
it does not delete branches. Merged PRs require a separate reviewed revert,
not this rollback mechanism.
Author rollback records its base-restoration ownership decision before restoring
the head, so interrupted recovery can resume without claiming a foreign base
change. An ambiguous original-head/release-base state requires manual review.

After publication, require the release ruleset checks, appropriate conformance,
renewed approval and resolution of **addressed** review conversations before
calling the migrated PR merge-ready. Start with one canary, then 3–5 PRs.

## Advisory guard and tooling tests

`pr-branch-guard.yaml` uses only trusted default-branch tooling with read-only
permissions. It reads PR diffs as data, produces routing guidance in the job
summary, and makes no labels/comments/base changes. Classification outages are
explicit warnings requiring manual review. It is advisory, not a required
branch-protection check.

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -B -m unittest discover \
  -s hack/pr-triage -p 'test_*.py'
actionlint .github/workflows/pr-branch-guard.yaml .github/workflows/check-pr-triage.yaml
```
