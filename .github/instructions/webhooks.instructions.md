---
applyTo: 'pkg/webhooks/**'
---

Read [`pkg/webhooks/AGENTS.md`](../../pkg/webhooks/AGENTS.md) in full before reviewing anything under this path —
treat it as ground truth, not this file. This is the latency-critical admission path; the file documents that it's
rule *type* (not enforce/audit mode) that decides synchronous vs. queued handling, and that a dry-run admission
request skips `UpdateRequest` creation entirely.
