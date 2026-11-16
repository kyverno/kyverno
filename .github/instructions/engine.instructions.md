---
applyTo: 'pkg/engine/**'
---

Read [`pkg/engine/AGENTS.md`](../../pkg/engine/AGENTS.md) in full before reviewing anything under this path —
treat it as ground truth, not this file. It documents behavior that doesn't show up in a diff: handler dispatch is
an if/else chain per rule (not a registry), `ApplyRules == ApplyOne` short-circuits after the first non-zero-applied
rule, and rules within the same policy see each other's patches via a checkpoint/restore of the JSON context.
