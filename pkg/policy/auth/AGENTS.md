# AGENTS.md — pkg/policy/auth

Auth pre-checks run during **policy admission validation** (not at resource-admission time). Given a
subject (service account) they answer "can this subject act on this resource?" by issuing Kubernetes
`SubjectAccessReview` requests, and surface the result as an admission warning or error.

## Layout

- `auth.go` — `AuthChecks` interface (`User()`, `CanI(ctx, verbs, gvk, namespace, name, subresource)`)
  and the real `Auth` implementation. `Auth.CanI` loops the verbs and calls `check` **once per verb**,
  so a single `CanI([]string{"get","list","watch"}, …)` fans out to **three** `SubjectAccessReview`
  requests. On any denied verb it aggregates the failures into one message via `buildMessage`.
- `cache.go` — `ResultCache` + the `cachedAuth` decorator (see contract below).
- `fake/` — `FakeAuth`, which approves everything; used on `mock`/offline (CLI) paths.

## ResultCache contract — do not loosen these

`cachedAuth` (via `NewCachedAuth(inner, cache)`) memoizes authorization decisions so the same check
isn't re-sent to the API server many times when one policy expands into many rules (autogen) and runs
the same reports/background checks per rule. Four invariants matter:

1. **Lifetime is one validation pass.** The cache is created in
   `pkg/validation/policy` `Validate()` and discarded when it returns. **Never** promote it to a
   longer-lived (controller- or process-scoped) cache: RBAC can change between passes, and a stale
   `allow`/`deny` would be a correctness/security bug. A single pass is synchronous and short, so no
   decision can go stale within it.

2. **Keying is per `(subject, verb, resource)` — one entry per verb, not per verb slice.** The key is
   `user | verb | gvk | namespace | name | subresource`. This granularity matches the one-SAR-per-verb
   fan-out in `Auth.CanI`, so an overlapping verb (e.g. `get`, shared by mutate's `{get,update}` and
   generate's `{get,create}`) is requested only once. If you ever key on the whole verb slice instead,
   overlapping calls stop deduping — the bug this cache was written to fix. `cachedAuth.CanI`
   reconstructs the aggregate message with the same `buildMessage` helper so the returned
   `(allowed, message)` is byte-identical to the uncached path.

3. **Errors are never cached; denials are.** A transient error must be retried on the next call. A
   deterministic `allow`/`deny` (including its message, which embeds only user/namespace/kind/verb) is
   safe to cache for the pass.

4. **It is NOT a single-flight cache.** Concurrent callers that miss the same key will each issue their
   own `SubjectAccessReview` before the first result is stored — the mutex only guards the map, it does
   not coalesce in-flight requests. Dedup is guaranteed only once a result has been recorded. This is
   fine because validation is sequential today; don't assume atomic single-flight if you parallelize.

Concurrency: `ResultCache` guards its map with a `sync.Mutex`. Validation is sequential today, so the
lock is defensive, not load-bearing — keep it anyway (a webhook could parallelize rule validation later).

Nil-safety: `NewCachedAuth` returns `inner` unchanged when either `inner` or `cache` is nil. Callers
depend on this — e.g. generate leaves its reports checker nil when `reportsSA == ""`, and the `mock`
path passes a nil cache so wrapping is a no-op.
