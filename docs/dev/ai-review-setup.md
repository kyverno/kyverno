# CodeRabbit + Copilot review setup

`.coderabbit.yaml`, `.github/copilot-instructions.md`, `.github/instructions/*.instructions.md`, and
`.github/skills/code-review/SKILL.md` are the file-based half of this repo's AI-review configuration — they're
committed, reviewed, and versioned like any other change. This doc is the other half: the settings that live in a
GitHub org/repo admin UI or a CodeRabbit dashboard, which no committed file can express, written as a runbook a
maintainer with the right access can work through directly. Nothing here is automated by anything in this repo.

Both bots are already active on every PR today, running off dashboard/platform defaults — confirmed live on
several real PRs (`coderabbitai[bot]` and `copilot-pull-request-reviewer[bot]` both post a review on every PR
checked; neither currently blocks a merge — every observed review state was `COMMENTED`, never `APPROVED` or
`CHANGES_REQUESTED`). The steps below turn on the parts of each platform's feature set that aren't file-based and
aren't already live.

## CodeRabbit dashboard (`app.coderabbit.ai`)

1. **Confirm current tier settings.** Sequence diagrams and the "Review Change Stack" link already appear on live
   PR walkthroughs without any committed config, which means CodeRabbit is already past a bare install and on
   Team-tier features (expected — Kyverno's OSS plan includes them). Use this step to confirm Triage and Change
   Stack are both actually turned on in the dashboard, not just nominally included in the plan.
2. **Triage → Slack digest.** Connect the `#maintainers` Slack channel and set a scheduled digest (Monday morning — adjust to whatever cadence maintainers actually want).
   This gives a ranked PR queue (risk, blast radius, review effort, suggested reviewer) without anyone visiting the dashboard. Not to be confused with the **Slack Agent** (see below) — a separate, conversational product.
3. ~~**Integrations → link `kyverno/api`**~~ — turns out this is YAML-configurable, not dashboard-only:
   `.coderabbit.yaml`'s `knowledge_base.linked_repositories` now links `kyverno/api` (the actual repo — not
   `kyverno-api`, a slug that doesn't exist; see
   [`docs/context/shared/repo-boundaries.md`](../context/shared/repo-boundaries.md)) directly, so a PR here that
   breaks the `policies.kyverno.io` CRD Go types externalized into that repo gets flagged during review. Nothing
   left to do on the dashboard for this one.
4. **Integrations → MCP servers**: **CodeGraph is likely unnecessary** — see
   [`ai-review-open-questions.md`](./ai-review-open-questions.md#question-5--is-the-codegraph-mcp-server-still-worth-building).
   CodeRabbit already runs a native Code Graph Analysis engine (on by default, no config) that covers same-repo
   caller/implementor lookups — the `kyctrl` prototype's plan to deploy a separate self-hosted CodeGraph MCP server
   for that would be redundant. `.coderabbit.yaml`'s `path_instructions` text has been updated accordingly. A
   CodeGraph MCP server would only still be worth building for something the native engine doesn't reach (e.g. a
   genuinely cross-repo call graph into `kyverno/api`) — open question, not a settled next step.
   - **DeepWiki** (`https://mcp.deepwiki.com/mcp`, no auth) and **Context7** remain independently useful — one-click
     from CodeRabbit's suggested-servers list, no dependency on CodeGraph.

## GitHub org settings → Copilot (org admin only)

5. **Code review → Approvals → "Allow Copilot to approve pull requests."** Currently **off** — confirmed live:
   PR #17572's Copilot review carries a 🟢 "Approval recommended" verdict in its comment body, but the review
   state on the GitHub Reviews API is `COMMENTED`, not `APPROVED` — meaning it's not yet a formal review that
   counts toward branch protection. Turning this on makes it one. Decide at the same time whether to restrict
   which paths count (e.g. require a Copilot approval on non-security paths but not on `api/**`/`pkg/webhooks/**`).
6. **Code review → default review effort → Balanced.** Balanced gives deeper reasoning on complex/security-sensitive
   code; Lite is targeted/cheaper. This is gated on confirming the Nirmata org's Copilot Business AI-Credit budget
   can absorb the higher per-review cost — unresolved as of this writing, needs an answer from whoever owns that
   plan before flipping this from whatever it's set to today.
7. **Repo Settings → Copilot → MCP servers**, once CodeGraph exists and is already wired into CodeRabbit (step 4):
   register the same URL here too. Copilot's MCP config is shared verbatim between code review and the Copilot Cloud Agent, so this is also what lets a maintainer assign a scoped issue to Copilot and have it open a PR with
   AST-grounded context. Auth tokens, if CodeGraph ever needs any, go under Settings → Secrets and variables → Agents, not this JSON block.

9. **Dependabot alerts → "Assign to Agent."** A per-alert action in the Security tab (distinct from
   `dependabot-autofix.yaml` below, which handles version-bump PRs with failing checks or merge conflicts) — this
   one hands an actual *vulnerability* alert to Copilot for remediation. Worth using given Kyverno's Dependabot
   volume, but it's a manual per-alert action a maintainer takes, not something that can be turned on once and
   forgotten. Same entitlement caveat as everything else on this page that needs the coding agent.

## Copilot coding agent is not currently usable in this org — confirmed, not assumed

Checked directly (2026-09-18): `@copilot` mentioned in a real PR comment
([kyverno/test-ai-assistants#2](https://github.com/kyverno/test-ai-assistants/pull/2)) produced no reaction, no
review, no commit. Ruled out a permissions problem (the commenter has `write`/`maintain` access, confirmed via the
API). Ran GitHub's own documented availability check —
`suggestedActors(capabilities: [CAN_BE_ASSIGNED])` via the GraphQL API — against both `kyverno/test-ai-assistants`
and `kyverno/kyverno`; `copilot-swe-agent` never appears in either, org-wide, not repo-specific. The repo-level
"Copilot cloud agent" settings page (Require approval for workflow runs / Allow automations / Only allow
write-access users) is a **policy guardrail page**, not the entitlement switch — it configures behavior for when
the coding agent is available, it doesn't grant it. The actual gap is almost certainly at org-wide Copilot policy
or Copilot Business/Enterprise seat entitlement for coding agent specifically — worth taking to whoever owns the
org's Copilot plan. **Copilot's *code review* is unaffected and works fine** — this is specifically about the
coding agent (the thing that would push a fix commit after an `@copilot` mention or issue assignment).

## Dependabot autofix — uses CodeRabbit, not Copilot, because of the above

- **`.github/workflows/dependabot-autofix.yaml`** — fully automatic, no maintainer comment involved. Triggers on
  `workflow_run` for each required check workflow completing; on a failure from a Dependabot PR, asks
  **CodeRabbit** to fix it via its documented comment command `@coderabbitai fix-ci commit`, capped per-check and
  per-PR. Merge conflicts are a separate concern, handled by `dependabot-rebase-on-conflict.yaml`
  (`@dependabot rebase`), not this file.
  **Confirmed, not assumed**: `.coderabbit.yaml`'s `auto_review.ignore_usernames` excludes `dependabot[bot]` from
  CodeRabbit's *automatic* review (rate-limit conservation), but that does not block *explicit* comment commands —
  CodeRabbit's own "Review skipped" message on a real Dependabot PR (#17570) says so directly: "Bot user detected.
  To trigger a single review, invoke the `@coderabbitai review` command." `@coderabbitai fix-ci commit` is the
  same kind of explicit command, so it isn't blocked by the exclusion. Still open: whether the CodeRabbit GitHub
  App installation actually has write access to push a fix commit — unconfirmed, and `dependabot-autofix.yaml`
  itself doesn't need `contents: write` either way (it only posts a comment; any push is CodeRabbit's own App
  acting under its own credentials, not this workflow's token). No CodeRabbit-authored commit has ever landed in
  this repo's history (`search/commits` for `author:coderabbitai[bot]` returns zero), which could mean no write
  access, or could just mean it's never been asked to push before `dependabot-autofix.yaml` existed — needs an org
  admin to check the App's installation permissions (Settings → GitHub Apps → CodeRabbit) to know which.

## MCP: CodeGraph — CodeRabbit first, then Copilot

Neither platform accepts a committed MCP config file — both are UI-entered (CodeRabbit: dashboard Integrations;
Copilot: repo Settings). Once CodeGraph is deployed and reachable, wire it to **CodeRabbit first**:

- CodeRabbit's MCP usage is steered by explicit `path_instructions` text (already written into this repo's
  `.coderabbit.yaml`) — a testable contract, not a hope. Copilot's MCP tool-calling during review is comparatively
  opportunistic.
- CodeRabbit's reviews are unlimited/free on the OSS plan; Copilot review already costs AI Credits per review, and
  that budget is an open question (see step 6) — adding MCP round-trips to every Copilot review is the wrong place
  to add query volume while that's unresolved.
- CodeRabbit's MCP registration needs no extra org-admin step beyond the dashboard entry; Copilot's needs repo
  Settings access and potentially an Agents secret.

Wire it to **Copilot second**, not never — once CodeGraph exists, the same server also backs the Copilot Cloud
Agent (issue → PR), which CodeRabbit has no equivalent to.

CodeGraph itself doesn't exist yet at any reachable URL as of this writing — it's `kyctrl` infrastructure, a
separate prototype, out of scope for this repo. This section exists so both registration steps are a 5-minute
dashboard/Settings entry when it does, not a redesign.

## Process changes to confirm before they go live

Two things in `.coderabbit.yaml` are new behavior for this repo, not just new tooling — a maintainer should say yes
before they take effect, not discover them after the fact:

- **`reviews.request_changes_workflow: true`** (with `pre_merge_checks` in `error` mode for the description
  requirement and the codegen gate) would be the **first blocking AI-review gate** in this repo's history —
  confirmed live, every CodeRabbit/Copilot review today is `COMMENTED`, never `CHANGES_REQUESTED`. This changes
  that for the two checks set to `error` mode.
- **The "Update CHANGELOG" post-merge action** appends to a new `## Unreleased` section it creates at the top of
  `CHANGELOG.md`. Today `CHANGELOG.md` is organized as committed `## vX.Y.Z` sections only, apparently updated at
  release-cut time — this is a process change to that convention, not just automation of an existing one.
- **`early_access: true`** would unlock CodeRabbit's early-access feature set — the docs only say "enables
  early-access features," with no scoped list of which ones, so treat it the same way: an explicit yes, not a
  default flip. Not currently set.

Anything that would have CodeRabbit push a commit *directly onto a contributor's existing PR branch* (`fix_ci`'s
commit variant, `resolve_merge_conflict`, `autofix`, `simplify`) is a separate, narrower category — tracked in
[`ai-review-open-questions.md`](./ai-review-open-questions.md), not here.

## Improvements — CodeRabbit's comment-driven code generation

`docs.coderabbit.ai/guide/improvements`: beyond flagging issues, CodeRabbit can generate the fix itself on
command — `@coderabbitai generate docstrings`, `generate tests`, `fix`. The mechanism: creates a new branch,
commits the change there, and opens its own PR — it does not touch the branch under review. That's why it isn't
in the open-questions doc: opening a new PR is fine under this repo's rule, only a direct push to the
contributor's own branch needs sign-off. `docstrings`/`unit_tests` are the two pieces of this already
`enabled: true` in `.coderabbit.yaml` (pre-existing).

## Finishing touches

One-click (or one-comment) actions available from a PR walkthrough. `enabled: true` only makes a finishing touch
*reachable* — none of them auto-fire on their own; a human or a workflow still has to trigger it via checkbox or
`@coderabbitai <command>`.

| Finishing touch | Trigger | Commits to the PR under review? | State |
|---|---|---|---|
| Docstrings | ✅ checkbox / `@coderabbitai generate docstrings` | No — opens a new PR | `enabled: true` |
| Unit tests | ✅ checkbox / `@coderabbitai generate unit tests` | No — opens a new PR | `enabled: true` |
| Fix CI | `@coderabbitai fix-ci` (stacked PR) / `fix-ci commit` (direct commit) | Only the `commit` variant | `enabled` not set — see open questions |
| Resolve merge conflict | `@coderabbitai resolve merge conflict` | Yes, inherently | `enabled` not set — see open questions |
| Autofix | 🪄 checkbox / `@coderabbitai autofix` | Yes | `enabled` not set — see open questions |
| Simplify | ✨ checkbox | Yes | `enabled` not set — see open questions |
| Custom recipes | `finishing_touches.custom[]` | Depends on the recipe | none defined |

## CodeRabbit comment commands

The full `@coderabbitai` command set (`guides/commands`, `reference/review-commands`), for reference:

`review`, `full review`, `pause`, `resume`, `ignore`, `summary`, `resolve`, `approve`, `autofix`,
`autofix stacked pr`, `local commit`, `fix-ci` / `fix-ci commit`, `resolve merge conflict`,
`generate docstrings`, `generate unit tests`, `generate sequence diagram`, `rate limit`, `configuration`,
`configuration override`, `generate configuration`, `emit path instructions`,
`generate project vocabulary`, `help`.

## Slack Agent (distinct from the Triage Slack digest above)

`docs.coderabbit.ai/overview/slack-agent`: a separate, conversational product from the Triage Slack digest in
step 2 above. The Slack Agent responds to `@coderabbit` mentions/DMs in Slack and can investigate ("ask questions
about your codebase, trace features, cross-reference errors with merged PRs"), plan (generate a coding plan from
a thread), and act ("ask CodeRabbit to open a pull request"). The Triage digest, by contrast, is a one-way-ish
scheduled summary with quick action buttons (approve/close/merge/request review). Both live under the same Slack
integration but are configured and used differently — not documented in this repo's config, dashboard-only.

## Change Stack

Reorganizes a large PR from a flat file list into logical cohorts/layers. On by default, no `.coderabbit.yaml`
needed for the base feature.

- **Navigation** — multi-view/layer/file traversal, search, deep-linking. Dashboard-only UI.
- **Chat** — Q&A pinned to a specific review snapshot (doesn't follow later pushes). Requires Team plan + write
  access to the PR; not available in Slack or PR comments.
- **Notifications** — posts PR lifecycle/change/conversation/CI events to a connected Slack or Discord thread
  (all-or-none, no per-event selection). Gated behind the `early_access` flag discussed above — not enabled.

## Knowledge base / Learnings

`docs.coderabbit.ai/knowledge-base`, `/knowledge-base/learnings`: Learnings are on by default, taught three ways —
a chat/PR-comment mention with a preference, `@coderabbitai add a learning using <file>`, or the
`app.coderabbit.ai/learnings` dashboard. `.coderabbit.yaml` now has an explicit `knowledge_base` block:
`code_guidelines.enabled: true` pointing at this repo's own `AGENTS.md`/`ARCHITECTURE.md`/`REVIEW.md` (a
first-class knowledge source instead of only duplicated across `path_instructions` text), `learnings.scope: auto`,
`opt_out: false`, and `linked_repositories` for `kyverno/api` (see step 3 above). Deliberately not setting
`learnings.approval_delay` — leaving it unset keeps learnings requiring manual admin approval rather than
auto-approving after N days with no review.

## Issue enrichment (available, not configured — informational only)

`docs.coderabbit.ai` documents automatic issue auto-labeling and (Team-tier) auto-generated implementation plans
for new/edited GitHub issues. Nothing in this repo's `.coderabbit.yaml` turns this on today — it's a distinct
feature surface from the PR-review `labeling_instructions` already configured, and out of scope for this pass.
Noted here for awareness if maintainers want it later; it writes to issues, not to a contributor's PR branch, so
it isn't in `ai-review-open-questions.md`.

## PR validation

`docs.coderabbit.ai/issues/pr-validation` covers whether a PR actually addresses what its linked issue asked —
this is the same thing `reviews.pre_merge_checks.issue_assessment` (currently `mode: warning`) already configures
in `.coderabbit.yaml`; no separate key exists.

## Related

- [`.coderabbit.yaml`](../../.coderabbit.yaml) — the file-based CodeRabbit config this runbook complements.
- [`.github/copilot-instructions.md`](../../.github/copilot-instructions.md),
  [`.github/instructions/`](../../.github/instructions/),
  [`.github/skills/code-review/SKILL.md`](../../.github/skills/code-review/SKILL.md) — the file-based Copilot
  review config.
- [`.github/workflows/dependabot-autofix.yaml`](../../.github/workflows/dependabot-autofix.yaml) — the
  CodeRabbit-based Dependabot-PR-fixing workflow.
- [`.github/workflows/copilot-cli-dependabot-autofix.yaml`](../../.github/workflows/copilot-cli-dependabot-autofix.yaml) —
  a parallel, Copilot-CLI-based version of the same idea, kept alongside `dependabot-autofix.yaml` so both can be
  evaluated against real PRs before picking one.
- [`.github/workflows/dependabot-rebase-on-conflict.yaml`](../../.github/workflows/dependabot-rebase-on-conflict.yaml) —
  merge-conflict handling for Dependabot PRs, shared by both of the above.
- [`REVIEW.md`](../../REVIEW.md) — the review checklist both configs point at rather than duplicate.
