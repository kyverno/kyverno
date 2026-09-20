---
applyTo: 'api/**'
---

Read [`api/AGENTS.md`](../../api/AGENTS.md) and
[`docs/context/shared/api-versioning.md`](../../docs/context/shared/api-versioning.md) in full before reviewing
anything under this path — treat them as ground truth, not this file. This directory holds this repository's own
local API types (`kyverno.io`, `wgpolicyk8s.io`, `reports.kyverno.io`), not the external `github.com/kyverno/api`
module (that's the separate `policies.kyverno.io` CEL-policy types). New `kyverno.io` types go to `v2alpha1` only,
and fields are additive-only within a version.
