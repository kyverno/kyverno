# AGENTS.md — pkg/auth/checker

The low-level authorization primitive. An `AuthChecker.Check(group, version, resource, subresource,
namespace, name, verb)` answers one `(identity, resource, verb)` question by issuing a single Kubernetes
access review:

- `NewSubjectChecker(client, user, groups)` → one `SubjectAccessReview` (an arbitrary subject).
- `NewSelfChecker(client)` → one `SelfSubjectAccessReview` (the caller's own identity).

`Check` is single-verb and expects the **already-resolved GVR** (group/version/resource); GVK→kind
resolution happens above this layer, in `pkg/auth` `RunAccessCheck`.

## ResultCache + cached decorators (`cache.go`)

`NewCachedChecker(inner, subject, groups, cache)` / `NewCachedSelfChecker(inner, cache)` wrap an
`AuthChecker` so repeated questions within one pass are answered from a shared `ResultCache`. This is
the canonical place the cache lives (it only ever touches `AuthChecker`); `pkg/policy/auth` and
`pkg/validation/policy` are consumers.

**Contract — do not loosen these:**

1. **Lifetime is one validation pass.** `ResultCache` has no expiry or invalidation. Create it once at
   the start of a pass (today: `pkg/validation/policy` `Validate()`) and discard it when that call
   returns. **Never** store it on a struct field, package var, or otherwise promote it to a
   controller-/process-scoped lifetime: it would serve stale `allow`/`deny` after an RBAC change — a
   correctness/security bug. `Test_Validate_DoesNotShareAuthCacheAcrossPasses` (in
   `pkg/validation/policy`) is the guard; a within-pass dedup counterpart lives beside it.

2. **The key is the full identity + resolved tuple:** `(self, subject, subjectGroups, group, version,
   resource, subresource, namespace, name, verb)`. A `SubjectAccessReview` is evaluated against the
   user *and its groups*, so `subjectGroups` is part of the identity — the same user with different
   groups keys separately. Groups are opaque bytes, so `identityGroups` encodes them injectively
   (sorted, each length-prefixed) — **do not** replace this with a delimiter join (`{"a\x00b"}` would
   alias `{"a","b"}`, and `[""]` would alias `[]`). `self` puts `SelfSubjectAccessReview` decisions in a
   domain disjoint from every subject.

3. **Errors are never cached; decisions (allow and deny) are.** A transient error must be retried.

4. **It is NOT single-flight.** Concurrent callers that miss the same key each issue their own review
   before the first result is stored — the mutex only guards the map. Fine while validation is
   sequential; don't assume atomic single-flight if you parallelize.

Nil-safety: both constructors return `inner` unchanged when either `inner` or `cache` is nil — callers
rely on a nil `inner` to skip checks, and the `mock`/offline CLI path passes a nil cache so wrapping is
a no-op.
