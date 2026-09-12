# Making kyverno/kyverno agent-friendly — status

Working note for an in-progress restructuring effort, not a finished proposal. See
[docs/designs/README.md](./README.md) for what belongs in this directory versus `kyverno/KDP`.

## Why

Make this repo's context legible to AI coding agents (and, as a side effect, new human contributors) without
hand-holding — in the spirit of Anthropic's [AI-native SDLC playbook](https://claude.com/blog/the-ai-native-sdlc-playbook)
and a Nirmata pilot spec the maintainer driving this shared as inspiration (kept locally as `AI_PLAN.MD`, not
committed — it describes a heavier process built for a 4-engineer proprietary repo, not this one). We're taking its
artifact model and repo layout (nested context files, ADRs, a designs/archive split) while rejecting its enforcement
machinery (mandatory CI gates, a "pilot owner" role) — this is a volunteer-maintained CNCF project, not a company
pilot, and the two research passes so far both confirmed the existing root `AGENTS.md` was a solid starting point,
not something to replace wholesale.

## A correction worth flagging explicitly

Early in this effort we (wrongly, per third-party blog claims) assumed Claude Code reads `AGENTS.md` natively. It
does **not** — confirmed directly from Anthropic's own docs: *"Claude Code reads CLAUDE.md, not AGENTS.md."*
Without a `CLAUDE.md` that imports it (`@AGENTS.md`), none of this content was actually reaching Claude Code
sessions, only tools with native AGENTS.md support (Codex, Cursor, etc.). This is now fixed (see below), but it's a
useful reminder that "the industry uses AGENTS.md" claims need per-tool verification, not just repetition.

Also verified and rejected along the way: a proposed `.agents/skills/` directory (claimed cross-tool support in
several SEO-blog search results) — checked directly against Claude Code's own skills docs, which read only from
`.claude/skills/`. Not adopted; no first-party source confirms `.agents/skills/` is real for any tool we checked.

## Done so far

1. **Hygiene fixes** — `AGENTS.md` and `CODEOWNERS` had several stale paths (`cmd/tools/`, `pkg/cosign`,
   `pkg/notary`, `pkg/controller/report`) pointing at code that moved or never existed under those names. Fixed.
2. **`ARCHITECTURE.md`** (new, root) — runtime boundaries per binary, `pkg/` dependency layering, sanctioned
   defaults (which client package to use, feature flags, event recording), confirmed generated/no-edit zones.
3. **`docs/context/shared/`** (new) — collapsed a real duplication (API-versioning rules existed almost verbatim in
   both `AGENTS.md` and `docs/dev/api/README.md`) into one canonical file, `api-versioning.md`. Added
   `client-access.md` (the `pkg/client` vs `pkg/clients` vs `dclient` distinction — genuinely undocumented before
   this) and `repo-boundaries.md` (every related `kyverno/*` repo — `api`, `KDP`, `website`, `community`,
   `chainsaw`, and the wider ecosystem — and why each is separate rather than a folder here). **Deliberately did
   not** add `logging.md`/`feature-flags.md` pointer files here: root `AGENTS.md` already links straight to
   `docs/dev/logging/logging.md` and `docs/dev/feature-flags/README.md` from its own Coding Conventions section, so
   a `docs/context/shared/` file for either would only redirect to a redirect — a real distinction (the
   `api-versioning`/`client-access` cases) justifies a shared file; matching the other two for structural symmetry
   alone doesn't.
4. **9 nested `AGENTS.md` files** — `api/`, `pkg/{engine,cel,webhooks,background,image,clients,toggle,controllers}/`.
   Each was rewritten after three parallel deep-read research passes that actually read source, tests, and recent
   git history (not directory listings), and every load-bearing claim was independently spot-checked against the
   real code in this session. This surfaced real, previously-undocumented facts an agent would otherwise have to
   rediscover the hard way — a sample:
   - `pkg/background` has **no** `DeletingPolicy` subpackage at all; DeletingPolicy runs as a self-scheduling
     reconcile loop in `pkg/controllers/deleting`, never via `UpdateRequest`.
   - `pkg/clients/dclient` (the client most policy-engine code actually uses) has no metrics/tracing/logging
     decorators of its own, unlike every generated client wrapper around it — but every controller binary
     constructs it through `cmd/internal.Setup`, which hands it already-instrumented dynamic/kube clients first, so
     ordinary delegated resource calls do carry that instrumentation. The real gap is narrower: `dclient` instances
     built outside that path (`cmd/cli/kubectl-kyverno`'s `apply` command, `ext/cluster.New`) get none at all.
   - `pkg/sigstoretuf` is a hand-rolled mutex working around a real upstream data race in sigstore's TUF client
     (kyverno/kyverno#15983); everything in the image-verification path fails closed, with one narrow,
     policy-author-declared exception.
   - The five CEL policy kinds (`vpol`/`mpol`/`gpol`/`dpol`/`ivpol`) do **not** share a uniform
     compiler/engine/autogen structure — `ivpol` has no compiler package of its own and reuses the legacy
     image-verification evaluator instead.
   - `api/kyverno/v2alpha1` currently holds one type (`GlobalContextEntry`), and its lifecycle is further along
     than a simple two-version promotion: `v2alpha1` (2024) → `v2beta1` (Oct 2025, current CRD storage version) →
     `v2` (Dec 2025, newest/stable-named but *not* yet the storage version) — a live example of the promotion
     lifecycle being messier in practice than "the stable-named version is the storage version" would suggest.
   - Two rows in `docs/dev/controllers/README.md` (`admission-report-controller`, `update-request-controller`) no
     longer match any `ControllerName` in the codebase — flagged in `pkg/controllers/AGENTS.md`, not yet fixed in
     that file itself.
5. **The `CLAUDE.md`/`GEMINI.md` loading-mechanism fix** (see correction above) — root `CLAUDE.md` and `GEMINI.md`,
   each a one-line `@AGENTS.md` import, plus a `CLAUDE.md` at every nested `AGENTS.md` location (Claude Code's
   subdirectory auto-load only triggers on the literal filename `CLAUDE.md`, not `AGENTS.md`). `CLAUDE.local.md`
   and `GEMINI.local.md` added to `.gitignore` pre-emptively for personal per-contributor overrides.
6. **`docs/designs/README.md`** (new) — the boundary-setting doc for this directory (scope vs. `kyverno/KDP`, vs. a
   future `docs/decisions/` ADR, vs. a future `docs/archive/`). `docs/archive/README.md` is still open — see
   "what's left" below.

## Deliberately not done (verified against evidence, not just deferred)

- **Nested `GEMINI.md` per package** — Gemini CLI's root-level `@file` import and `context.fileName` settings are
  confirmed real, but its *nested-directory* auto-load behavior (the Claude-Code-specific mechanic that justifies
  per-package `CLAUDE.md` files) isn't confirmed. Root `GEMINI.md` only, for now.
- **`.claude/rules/` path-scoped rules** — a real, documented Claude Code mechanism, but everything it would cover
  here (package-level conventions) is already covered by the nested `AGENTS.md` files, which are also read by
  non-Claude tools. Would be genuinely useful for a file-*pattern*-scoped concern (e.g. all `*_test.go`) if one
  comes up; not adding it speculatively.
- **`.agents/skills/`** — not real for Claude Code (see correction above). Skills stay at `.claude/skills/`,
  clearly labeled Claude-specific.

## What's left (steps 5–10 of the original plan)

5. `docs/context/{features,components}/` — per-feature (cross-package, user-outcome-level: image verification,
   generate policies, background cleanup, policy exceptions, CEL migration, reporting) and per-component
   (package-level, one per `CODEOWNERS` domain) context docs. ~17 files. Needs the same code-grounded treatment as
   step 4, not a quick pass.
6. `docs/decisions/` — ADR template plus 3 backfilled ADRs: the CEL engine split, the `pkg/clients` instrumented
   wrapper layer, and `policies.kyverno.io`'s externalization to `kyverno/api`.
7. `docs/archive/README.md` — the boundary-setting doc for that not-yet-created directory (`docs/designs/README.md`
   is already done — see "Done so far" above). The archive doc carries a maintainer-decision list (stale
   `CHANGELOG.md`, unmaintained `litmuschaos/`, likely-dead `docs/crd/v1/index.html`, a duplicate template pair in
   `docs/user/{html,template}/`) rather than moving anything unilaterally.
8. `REVIEW.md`, `CONTRIBUTING-AGENTS.md` (how AI-assisted PRs work here — the org already has an `AI_USAGE_POLICY.md`
   in `kyverno/community` and recent commits already carry `Assisted-by: Claude` trailers; this repo just doesn't
   document the convention yet), and an optional advisory `Change: intent=/risk=/context=` line in the PR template
   (unparsed by CI — a human/agent self-classification aid, not a gate).
9. `.claude/settings.json` — `permissions.deny` enforcing the confirmed generated/no-edit zones by path (works
   regardless of the missing `DO NOT EDIT` header on `pkg/clients/**/*.generated.go`), plus one non-blocking `Stop`
   hook that nudges toward keeping `docs/context/` current once `docs/context/INDEX.md` exists.
10. Three skills: `pkg/toggle/.claude/skills/add-feature-flag/`, `test/conformance/chainsaw/.claude/skills/write-chainsaw-test/`
    (co-located per Claude Code's own recommended pattern), and a root-level `.claude/skills/review-pr/` pointing at
    `REVIEW.md`.

## Explicitly out of scope, this whole effort

CI-enforced classification parsing, an "AI review gate" bot, `docs/context` freshness CI checks, and merging
`kyverno/api` back into this repo (a real release-engineering decision, not a docs-structure one — the split exists
so external Go projects can import Kyverno's types without the whole controller dependency tree; see
`docs/context/shared/repo-boundaries.md`). All flagged as follow-ups needing explicit maintainer buy-in, not shipped
here.
