# Feature flags

Canonical doc: [docs/dev/feature-flags/README.md](../../dev/feature-flags/README.md). This file is a pointer, not
a copy — edit rules there. Step-by-step "how do I add one correctly" also lives in
[pkg/toggle/AGENTS.md](../../../pkg/toggle/AGENTS.md). There's no co-located skill for this yet (tracked as future
work in [docs/designs/agent-friendly-restructure.md](../../designs/agent-friendly-restructure.md)).

Quick summary: `pkg/toggle` is the preferred mechanism — env var + CLI flag + a method on the `Toggles`
interface (`pkg/toggle/context.go`), read at call sites via `toggle.FromContext(ctx).<Feature>()`. A simpler,
secondary pattern (container-arg-only flags bypassing `pkg/toggle`) is documented in the canonical doc for
straightforward on/off cases — prefer the `Toggles` interface pattern unless you have a specific reason not to.
