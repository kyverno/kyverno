# CodeRabbit: open questions for the mentor

`.coderabbit.yaml` deliberately leaves a handful of CodeRabbit capabilities disabled, not because they're bad
ideas, but because each one lets CodeRabbit push a commit **directly onto a contributor's existing PR branch** —
that's a decision a mentor should sign off on before it goes live, not something to flip on unilaterally. Opening
a brand-new PR, writing to an issue, or auto-labeling isn't gated the same way (see
[`ai-review-setup.md`](./ai-review-setup.md) for those — they're documented as available/implemented directly).

## Why these specific ones are gated

Each command below is a single `.coderabbit.yaml` `enabled` flag that, once on, is reachable repo-wide by anyone
who types the right `@coderabbitai` comment on any PR — not just on the PR a maintainer intended. There's no way
to scope the flag itself to "only Dependabot PRs" or "only when a workflow sends it"; that scoping has to happen
outside the flag (a workflow that sends the comment only in the right conditions, or just relying on nobody typing
it inappropriately).

| Command | What it does | Touches the contributor's existing branch? |
|---|---|---|
| `@coderabbitai fix-ci` (no `commit`) | Opens a **new stacked PR** with the CI fix | No — safe, not gated |
| `@coderabbitai fix-ci commit` | Commits the CI fix **directly to the current PR branch** | **Yes — gated** |
| `@coderabbitai resolve merge conflict` | Resolves the conflict via a commit to the current branch (no non-commit variant exists — the fix has to land where the conflict is) | **Yes — gated** |
| `@coderabbitai autofix` | Applies CodeRabbit's own inline suggestions as a commit to the current PR | **Yes — gated** |
| `@coderabbitai simplify` (via the ✨ checkbox) | Applies simplification suggestions as a commit to the current PR | **Yes — gated** |
| Custom `finishing_touches.custom[]` recipes | Depends entirely on how the recipe's `instructions` text is written | **Depends — write it carefully if we add one** |

For contrast, `@coderabbitai generate docstrings` / `generate unit tests` / the general `@coderabbitai fix`
(`guide/improvements`) all create a **new branch and open their own PR** rather than touching the branch under
review — not gated by this rule. Those are already `enabled: true` in `.coderabbit.yaml` today (pre-existing, not
something this pass turned on).

`fix_ci` and `resolve_merge_conflict` are each a single flag that also unlocks their unsafe variant — you can't
enable just the "opens a stacked PR" behavior of `fix-ci` via YAML without also making `fix-ci commit` reachable.

## Question 1 — enable `fix_ci` at all?

`.github/workflows/dependabot-autofix.yaml` already sends `@coderabbitai fix-ci commit` today, scoped to
`dependabot[bot]`-authored PRs only — but `finishing_touches.fix_ci.enabled` is not actually set anywhere in
`.coderabbit.yaml`, and it defaults to `false`. **That workflow's comment is likely silently no-op'ing right
now.** Turning the flag on would fix that — but repo-wide, meaning a human could also type the same command on
their own PR (or, in principle, on someone else's).

Already confirmed, not a guess: `.coderabbit.yaml`'s `auto_review.ignore_usernames` (which excludes
`dependabot[bot]` from *automatic* review) does **not** block this *explicit* comment command — CodeRabbit's own
"Review skipped" message on a real Dependabot PR (#17570) confirms bot-authored PRs can still be reached via an
explicit `@coderabbitai <command>`. So the `fix_ci.enabled` flag being off is the one remaining, confirmed reason
this doesn't work today — not an open question anymore.

Still open: whether CodeRabbit restricts these commands to users with write access to the repo — if so,
"repo-wide" in practice means "maintainers only," which changes how much risk enabling this actually carries.

Note there's also a separate, already-built, non-CodeRabbit path doing something similar today:
`.github/workflows/copilot-cli-dependabot-autofix.yaml` uses GitHub Copilot CLI (not CodeRabbit) to read a
failing check's log and push a fix commit directly to the Dependabot PR branch, with its own `contents: write`
token — also bot-PR-only. Per that file's own header comment, it's being kept alongside
`dependabot-autofix.yaml` so both can be evaluated before picking one. Worth folding into this same mentor
conversation rather than deciding the CodeRabbit path in isolation.

**Decide:** keep `dependabot-autofix.yaml`'s bot-only design as the only way `fix_ci commit` gets triggered
(confirms it's still what's wanted, now that it would actually start working), and separately —

## Question 1b — `resolve_merge_conflict`

Unlike `fix_ci`, **no existing workflow relies on this today.** Dependabot PR merge conflicts are already handled
by `.github/workflows/dependabot-rebase-on-conflict.yaml`, which uses Dependabot's own native
`@dependabot rebase` command — not CodeRabbit at all. So enabling `finishing_touches.resolve_merge_conflict` would
be a genuinely new capability, not a bug fix for something already relied on. Same commit-to-branch property as
`fix_ci`, same (a)/(b) choice below if there's a reason to want it (e.g. for human-PR conflicts, which
`dependabot-rebase-on-conflict.yaml` doesn't cover since it only searches `author:app/dependabot`).

## Question 2 — should this ever extend to human contributor PRs, and how?

Two ways to do it, if the answer is yes:

- **(a) Automate it** — a GitHub Actions workflow that comments the `@coderabbitai fix-ci commit` (or
  `resolve merge conflict`) trigger automatically, e.g. on a CI failure for any PR, the same pattern
  `dependabot-autofix.yaml` already uses for bots, just without the `dependabot[bot]`-author restriction.
- **(b) Leave it purely on-demand** — enable the flag so the command exists, but build no workflow around it; a
  maintainer/admin types it by hand only when they judge it appropriate for that specific PR.

No workflow for human PRs exists today, and none is being built speculatively — this needs the mentor's call
first.

## Question 3 — `autofix` / `simplify`

Net-new capabilities, not used by any current workflow or command. Same "commits directly to the branch under
review" property as above. Same (a)/(b) choice if we want them at all.

## Question 4 — custom finishing-touch recipes

Two ideas floated earlier (not committed to `.coderabbit.yaml`): a "run `make codegen-all-code` and commit" recipe
and an "add a chainsaw conformance test stub" recipe. Both would need their `instructions` text written carefully
if they're meant to commit in place — same sign-off as the others.

## Question 5 — is the CodeGraph MCP server still worth building?

Separate from all of the above (this one isn't about pushing commits — it's about whether the underlying
motivation for the `kyctrl` prototype's CodeGraph MCP server still holds). Research into CodeRabbit's own docs
found it already runs a native "Code Graph Analysis" engine, on by default, with no MCP server needed — it builds
a map of every node a change affects and understands cross-file relationships as part of normal PR review
(`docs.coderabbit.ai/guides/code-review-overview`; multiple `docs.coderabbit.ai/changelog` entries expanding this
per-language with "no new setting or migration" required). `.coderabbit.yaml`'s `path_instructions` text has
already been updated to stop assuming an external MCP server is needed for same-repo caller/implementor lookups.

**Decide:** is there still a real use case for a separate CodeGraph MCP server — e.g. genuinely cross-repo call
graphs into `kyverno/api`, or git-history-aware queries the native engine doesn't do — or should that part of the
`kyctrl` plan be dropped?

## Related, already flagged elsewhere (not duplicated here)

These aren't branch-push actions, so they don't belong in this doc, but they're also already-live process changes
worth the mentor confirming — see the "Process changes to confirm before they go live" section in
[`ai-review-setup.md`](./ai-review-setup.md): `request_changes_workflow: true` (first blocking AI-review gate in
this repo's history), the "Update CHANGELOG" post-merge action (commits directly to `main` after a qualifying
merge), and `early_access: true` (an undocumented-scope beta flag, not currently set).
