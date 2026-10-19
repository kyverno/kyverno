# AGENTS.md — pkg/policy/auth

Auth pre-checks run during **policy admission validation** (not at resource-admission time). Given a
subject (service account) they answer "can this subject act on this resource?" by issuing Kubernetes
`SubjectAccessReview` requests, and surface the result as an admission warning or error. The
`validate.cel` path additionally issues `SelfSubjectAccessReview`s to check VAP/VAPB generation
permissions.

## Layout

- `auth.go` — `AuthChecks` interface (`User()`, `CanI(ctx, verbs, gvk, namespace, name, subresource)`)
  and the real `Auth` implementation. `Auth.CanI` loops the verbs and calls `check` **once per verb**,
  so a single `CanI([]string{"get","list","watch"}, …)` fans out to **three** access reviews. On any
  denied verb it aggregates the failures into one message via `buildMessage`. `NewAuth` takes an
  optional `*ResultCache`; when non-nil it builds the subject checker once and wraps it with
  `NewCachedChecker`.
- `cache.go` — `ResultCache` + the `cachedChecker` decorator (see contract below).
- `fake/` — `FakeAuth`, which approves everything; used on `mock`/offline (CLI) paths.

## Why the cache wraps `checker.AuthChecker`, not `AuthChecks.CanI`

The cache decorates the lowest primitive, `pkg/auth/checker.AuthChecker.Check(group, version, resource,
…, verb)` — the same seam upstream Kubernetes caches at in `NewCachingAuthorizer` — rather than the
higher `AuthChecks.CanI` boundary. That matters:

- **It keys on the resolved GVR.** GVK→GVR resolution happens in `pkg/auth` `RunAccessCheck` *before*
  the checker runs, so `"ConfigMap"` and `"v1/ConfigMap"` collapse onto one entry. A `CanI`-level cache
  keyed on the raw kind string would treat them as distinct.
- **No duplicated logic.** `Auth.CanI` + `buildMessage` run unchanged on top, so the aggregate
  `(allowed, message)` is byte-identical to the uncached path — the cache layer reconstructs nothing.
- **The inner `AuthResult` (including `Reason`) is returned verbatim**, correct for the
  `RunAccessCheck`-direct callers (engine adapter, cleanup policy) that surface `Reason`.
- **The `validate.cel` self-checks are covered too**, because they also go through `checker.AuthChecker`
  (`pkg/validation/policy/actions.go` wraps `NewSelfChecker` with `NewCachedSelfChecker`).

## ResultCache contract — do not loosen these

`cachedChecker` memoizes `Check` results. Five invariants matter:

1. **Lifetime is one validation pass.** The cache is created in `pkg/validation/policy` `Validate()`
   and discarded when it returns. **Never** promote it to a longer-lived (controller- or
   process-scoped) cache: RBAC can change between passes, and a stale `allow`/`deny` would be a
   correctness/security bug. A single pass is synchronous and short, so no decision can go stale within
   it.

2. **Keying is per `(self, subject, resolved-GVR, subresource, namespace, name, verb)`** — a struct key
   (`cacheKey`), one entry per verb. The per-verb granularity matches the one-review-per-verb fan-out in
   `Auth.CanI`, so an overlapping verb (e.g. `get`, shared by mutate's `{get,update}` and generate's
   `{get,create}`) is requested only once. `subject` is the SubjectAccessReview user; it keeps
   different identities from colliding in a shared cache. `self` marks `SelfSubjectAccessReview`
   decisions so they occupy a **disjoint key domain** from every subject — a self decision is never
   reused for a subject even if a service account is literally named like the self value.

3. **Errors are never cached; decisions are.** A transient error must be retried on the next call. A
   deterministic `allow`/`deny` is safe to cache for the pass.

4. **It is NOT a single-flight cache.** Concurrent callers that miss the same key will each issue their
   own access review before the first result is stored — the mutex only guards the map, it does not
   coalesce in-flight requests. Dedup is guaranteed only once a result has been recorded. This is fine
   because validation is sequential today; don't assume atomic single-flight if you parallelize.

Concurrency: `ResultCache` guards its map with a `sync.Mutex`. Validation is sequential today, so the
lock is defensive, not load-bearing — keep it anyway (a webhook could parallelize rule validation later).

Nil-safety: `NewCachedChecker` / `NewCachedSelfChecker` return `inner` unchanged when either `inner` or
`cache` is nil. Callers depend on this — e.g. generate leaves its reports checker nil when
`reportsSA == ""`, and the `mock` path passes a nil cache so wrapping is a no-op.
