---
applyTo: 'api/**'
---

Read [`api/AGENTS.md`](../../api/AGENTS.md) and
[`docs/context/shared/api-versioning.md`](../../docs/context/shared/api-versioning.md) in full before reviewing
anything under this path — treat them as ground truth, not this file. This is the external
`github.com/kyverno/api` module boundary: new types go to `v2alpha1` only, and fields are additive-only within a
version.
