---
name: code-review
description: Kyverno-specific review guidance for pull requests. Use whenever reviewing a PR against kyverno/kyverno — points to the project's real review checklist and per-package context instead of generic review practice.
---

Before reviewing, apply [`REVIEW.md`](../../../REVIEW.md) — the project-specific checklist of things that are easy
to miss from a diff alone (DCO trailers, generated/no-edit zones, codegen freshness across both code and docs, the
4-file CRD schema fan-out, the `pkg/toggle` per-binary flag-registration gotcha, and which CI checks do and don't
run automatically on a PR). It is maintained separately from this skill; don't restate its content here, read it.

For package-specific behavior not visible from a diff, read that package's own `AGENTS.md` before reviewing — this
repo has real, already-verified ones for `pkg/engine/`, `pkg/webhooks/`, `pkg/cel/`, `pkg/clients/`, `pkg/toggle/`,
`pkg/controllers/`, `pkg/background/`, `pkg/image/`, and `api/`. `.github/copilot-instructions.md` and
`.github/instructions/*.instructions.md` already carry the pointers for the highest-traffic ones
(`pkg/engine/`, `pkg/webhooks/`, `api/`) — this skill is the fallback for anywhere else in the tree.

**Lane separation**: CodeRabbit also reviews every PR on this repo, focused on security vulnerabilities, linting,
and style. Prioritize logic correctness, cross-file impact, and architectural concerns that need full-repository
context to detect, rather than duplicating what a linter or CodeRabbit would already flag.

**Architecture context**: consult [`ARCHITECTURE.md`](../../../ARCHITECTURE.md) for the domain layering (legacy
`pkg/engine` vs. the CEL `pkg/cel` stack) and its Generated/no-edit-zones table before flagging a change to a
generated file as a hand-edit rather than a missing `make codegen-*` run.
