# AGENTS.md — pkg/cel

The CEL-based engine stack for `policies.kyverno.io` types (ValidatingPolicy/`vpol`, MutatingPolicy/`mpol`,
GeneratingPolicy/`gpol`, DeletingPolicy/`dpol`, ImageValidatingPolicy/`ivpol`). Sits alongside `pkg/engine`, not
layered under it. **The Go types themselves are not in this repo** — see [api/AGENTS.md](../../api/AGENTS.md) and
[docs/context/shared/api-versioning.md](../../docs/context/shared/api-versioning.md).

## The five policy kinds do NOT share one structural template — verified, not assumed

| kind | `compiler/` | `engine/` | `autogen/` | other |
|---|---|---|---|---|
| vpol | yes | yes | yes | — |
| mpol | yes | yes | yes | — |
| gpol | yes | yes | **no** | `template/` — custom `(( ))` YAML-placeholder scanner (see below) |
| dpol | yes | yes | **no** | — |
| ivpol | **no** | yes | yes | reuses `pkg/image/verification/evaluator` as its "compiler" |

If you're adding a **6th kind**, don't copy the vpol layout by reflex — decide per-kind whether autogen even applies
(dpol/gpol say no: dpol validates a cron schedule directly with the same parser the deleting controller uses; gpol
generates from YAML templates, not pod-controller rule replication) and whether to build a fresh `compiler`
subpackage or reuse an existing evaluator the way ivpol reuses the legacy image-verification one.

**ivpol is the deliberate structural outlier.** It has no `pkg/cel/policies/ivpol/compiler` and no `validate.go`.
Its compile/validate entry points are `pkg/image/verification/evaluator` (shared with *legacy*, non-CEL image
verification) — `pkg/webhooks/policy/handlers.go` calls `eval.Validate(ivpol, ...)` directly, not a
`pkg/cel/policies/ivpol.Validate()`. A comment in `ivpol/engine/engine.go` notes its evaluation logic is
deliberately kept parallel to vpol's even though it lives in a different underlying package. This split is also
where most recent CEL-related bugs have clustered (`bb09b2a15`, `f0c5d57c5`, `17fbbf48b`) — treat ivpol changes with
extra care and check both the CEL and legacy image-verification paths.

## Compilation and evaluation

Each kind has its own `compiler` package building a dedicated CEL `cel.Env`, wired to Kyverno's CEL extension
libraries (`github.com/kyverno/sdk/extensions/cel/libs/*` — globalcontext, gzip, hash, http, image, imagedata, json,
math, random, resource, time, transform, user, x509, yaml — plus this repo's own `pkg/cel/libs`). The engine
abstraction is one generic interface, `Engine[T any] { Handle(ctx, EngineRequest, predicate func(T) bool) }`,
instantiated per kind. `pkg/cel/libs/context.go` is the glue between CEL and live cluster state — it wires
`pkg/clients/dclient`, `pkg/globalcontext/store`, and `pkg/background/common` into CEL's `resource.Get`,
`globalContext.get`, and generator functions.

**Namespaced policies cannot use `globalContext`** — a namespaced `ValidatingPolicy` whose expression calls
`globalContext.get(...)` fails validation with an explicit error (see `vpol/validate_test.go`'s
`Test_Validate_NamespacedPolicyRejectsGlobalContext`). Global context is cluster-scoped external data; deliberately
disallowed at namespace scope.

## `pkg/cel/autogen` is a separate reimplementation of `pkg/autogen`, not a shared library

Structurally parallel to (not code-shared with) the legacy `pkg/autogen`. Actively being generalized beyond built-in
pod-controller kinds to custom workload CRDs (e.g. JobSet) — check recent commits before assuming the autogen
surface is fixed.

## `gpol`'s generation model is templates, not rule replication

`pkg/cel/policies/gpol/template` implements YAML template generation with `(( ... ))` CEL-expression placeholders: a
placeholder occupying an entire scalar value is spliced structurally; one embedded in a larger string is
interpolated and must evaluate to a scalar; placeholders are not supported in mapping keys. This is why gpol has a
`template/` package instead of `autogen/` — its generation mechanism is fundamentally different from vpol/mpol's.

## PolicyException CEL match-conditions are grandfathered, not retroactively re-validated

`pkg/cel/compiler/policy_exception.go`'s `CompilePolicyExceptionMatchConditions` caps `matchConditions` at 64
entries and compiles **already-stored** expressions (from the exception's previous version) under a looser
`StoredExpressions` environment while brand-new expressions get the stricter `NewExpressions` environment.
Tightening CEL validation rules later won't retroactively break an already-admitted `PolicyException` unless its
expression text itself changes.

## Recent activity worth knowing about

CEL engine dependency itself has had a security fix (`2e9ef352e`, bumping `cel-go` for `GHSA-gcjh-h69q-9w9g` — the
GitHub Security Advisory's canonical identifier; it has no assigned CVE, so don't prefix it with `CVE-`).
`PolicyExceptions` are read from the manager cache, not a second informer (`47f45aebe`) — don't reintroduce a
second informer for exceptions. `globalContext` is explicitly denied in namespaced policies (`795394e1c`, see
above). Namespace scoping/routing for the newer namespaced CEL policy variants has had several recent bugs — see
[pkg/webhooks/AGENTS.md](../webhooks/AGENTS.md).
