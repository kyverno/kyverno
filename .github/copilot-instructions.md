Kyverno is a Kubernetes-native policy engine for security, compliance, and governance via admission control and
background scans. This file is instructions for *how* to review here — facts about specific packages, CI behavior,
and process live in the files it points to below; don't restate their content here, read them.

## Priorities

1. **Policy evaluation correctness is the highest priority.** A wrong evaluation can incorrectly block or allow a
   workload in a cluster. Prefer flagging a plausible correctness issue over staying silent on it.
2. **The admission webhook path (`pkg/webhooks/**`) is latency-critical.** Flag any new synchronous blocking
   operation without a timeout, missing panic recovery, or anything that could block admission incorrectly.
3. **`api/**` holds this repository's own local API types** — `kyverno.io`, `wgpolicyk8s.io`
   (as `policyreport`), and `reports.kyverno.io`. New `kyverno.io` types go to `v2alpha1` only, never directly to
   `v1`/`v2`. The CEL-policy types (`policies.kyverno.io`) are the ones that live in the separate
   `github.com/kyverno/api` module (see `docs/context/shared/repo-boundaries.md`) — judge backward compatibility
   against external consumers only for those, not for anything under this repo's own `api/`.

## Before reviewing a package: read its `AGENTS.md`

GitHub Copilot code review is confirmed to read only the *root* `AGENTS.md` automatically, not the nested
per-package ones this repo has. When a PR touches one of these, explicitly fetch and read that file first — it
documents non-obvious behavior (dispatch order, what's deliberately *not* implemented, generated-file boundaries,
concurrency constraints) that a diff alone won't show, and judging the diff without it risks flagging intentional
behavior as a bug, or missing a real one that contradicts it:

`pkg/engine/AGENTS.md`, `pkg/webhooks/AGENTS.md`, `pkg/background/AGENTS.md`, `pkg/controllers/AGENTS.md`,
`pkg/cel/AGENTS.md`, `pkg/image/AGENTS.md`, `pkg/clients/AGENTS.md`, `pkg/toggle/AGENTS.md`, `api/AGENTS.md`.

If a package under review has no `AGENTS.md`, that itself is worth a low-severity comment suggesting one, rather
than silently proceeding as if there were nothing non-obvious to know.

## How to review, not just what to flag

- **Cite specifics.** A comment that references the actual file/line/function it's about is actionable; a general
  "consider improving error handling" is not — prefer the former, or don't comment.
- **Check whether the behavior is already documented as intentional before flagging it as a bug.** Several
  packages here have deliberately unusual designs (e.g. a resource type with no handling in a package where you'd
  expect it, a lock that isn't re-entrant, a cache with no explicit invalidation) — the relevant `AGENTS.md`
  explains why when that's the case. Contradicting a documented design without addressing *why* it was chosen is a
  weaker comment than one that engages with the stated reasoning.
- **Distinguish blocking from advisory.** Say plainly whether something must change before merge (a correctness or
  security issue) versus something worth considering (a readability/structure suggestion) — don't let severity
  language drift toward "must" for the latter.
- **When a CodeGraph MCP server is connected**, query it for callers/implementors of any modified function or
  interface before asserting a cross-package impact — cite the exact file:line it returns, don't guess at blast
  radius from the diff alone.
- **Prefer a question over an assertion when the repo-specific reasoning genuinely isn't visible** — e.g. "is this
  intentional given X?" surfaces the same concern without misrepresenting confidence.
- **Don't flag a hand-edit to a generated file as ordinary content** — check [`ARCHITECTURE.md`](../ARCHITECTURE.md)'s
  Generated/no-edit-zones table first; the right comment there is "run the corresponding `make codegen-*` target,"
  not a content review of the generated output.

## For everything else

Apply [`REVIEW.md`](../REVIEW.md) — the project's own maintained review checklist (DCO, codegen-freshness CI
mechanics, the CRD multi-file fan-out, feature-flag registration, what CI does and doesn't run automatically on a
PR, the AI-assisted-PR disclosure norm, and more). It's kept separately from this file specifically so there's one
source of truth instead of two copies drifting apart — read it, don't expect its content mirrored here.

## Lane separation with CodeRabbit

This PR is also reviewed by CodeRabbit — except on a Dependabot-authored PR, which `.coderabbit.yaml` configures
it to skip entirely. Where CodeRabbit is reviewing too, it's instructed to focus on security vulnerabilities,
linting/static-analysis findings, and style; to avoid duplicate or conflicting comments, focus here on logic
correctness, cross-file impact, and architectural concerns that need full-repository context to see, and don't
re-flag something CodeRabbit already commented on in this same review round.
