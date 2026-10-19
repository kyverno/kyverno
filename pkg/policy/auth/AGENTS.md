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

## ResultCache contract

The cache and its "do-not-loosen" contract (one-pass lifetime, never process-scoped or it serves stale
RBAC; identity = subject + groups + self; errors not cached; not single-flight) live with the code in
[`pkg/auth/checker`](../../auth/checker/AGENTS.md). The one consumer-side invariant to preserve here:
`NewAuth` must build a fresh cache per validation pass and never retain it on a long-lived struct.
