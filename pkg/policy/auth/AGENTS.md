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
  optional `*checker.ResultCache`; when non-nil it builds the subject checker once and wraps it with
  `checker.NewCachedChecker`.
- `fake/` — `FakeAuth`, which approves everything; used on `mock`/offline (CLI) paths.

The memoizing decorator itself lives in **`pkg/auth/checker`** (`cache.go`:
`ResultCache` + `cachedChecker`, `NewCachedChecker`/`NewCachedSelfChecker`), next to the
`checker.AuthChecker` it wraps. GVK→GVR resolution happens in `pkg/auth` `RunAccessCheck` before the
checker runs, so the cache keys on the resolved GVR — `"ConfigMap"` and `"v1/ConfigMap"` collapse onto
one entry.

## ResultCache contract — do not loosen these

1. **Lifetime is one validation pass.** The cache is created in `pkg/validation/policy` `Validate()`
   and discarded when it returns. **Never** promote it to a controller- or process-scoped cache: RBAC
   can change between passes, and a stale `allow`/`deny` would be a correctness/security bug.

2. **Keying is per `(self, subject, subjectGroups, resolved-GVR, subresource, namespace, name, verb)`**
   — one entry per verb, matching the one-review-per-verb fan-out in `Auth.CanI`, so an overlapping verb
   (`get`, shared by mutate's `{get,update}` and generate's `{get,create}`) is requested once. `subject`
   plus `subjectGroups` are the full SubjectAccessReview identity (a review is evaluated against the user
   *and* its groups), so the same username with different groups keys separately. `self` puts
   `SelfSubjectAccessReview` decisions in a disjoint domain, so a self decision is never reused for a
   subject even if a service account is literally named like the self value.

3. **Errors are never cached; decisions are.** A transient error must be retried on the next call.

4. **It is NOT a single-flight cache.** Concurrent callers that miss the same key each issue their own
   access review before the first result is stored — the mutex only guards the map. Fine while
   validation is sequential; don't assume atomic single-flight if you parallelize.

Nil-safety: `NewCachedChecker` / `NewCachedSelfChecker` return `inner` unchanged when either `inner` or
`cache` is nil. Callers depend on this — generate leaves its reports checker nil when `reportsSA == ""`,
and the `mock` path passes a nil cache so wrapping is a no-op.
