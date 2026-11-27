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

`mpol/compiler.Compile` dispatches on evaluation mode, like `vpol`. JSON-mode
mutation uses the existing provider/engine with `EngineRequest.JsonPayload` and
object-root `unstructured.Unstructured` inputs and outputs. Policies use the
normal in-cluster lifecycle, including policy validation and resource matching;
there is no mutation-specific JSON isolation layer.
See [the JSON mutation proposal](../../docs/designs/json-mutation.md) for the contract.

Four of the five kinds have their own `compiler` package building a dedicated CEL `cel.Env` (`ivpol` is the outlier
— see above); each is wired to Kyverno's CEL extension
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

## `pkg/cel/autogen` is the only autogen implementation

The legacy `pkg/autogen` (autogen for `kyverno.io` rules) was deleted in #17710 PR 4; `pkg/cel/autogen` is now the
sole autogen implementation, for `policies.kyverno.io` types only. It is actively being generalized beyond
built-in pod-controller kinds to custom workload CRDs (e.g. JobSet) — check recent commits before assuming the
autogen surface is fixed.

## `gpol`'s generation model is templates, not rule replication

`pkg/cel/policies/gpol/template` implements YAML template generation with `(( ... ))` CEL-expression placeholders: a
placeholder occupying an entire scalar value is spliced structurally; one embedded in a larger string is
interpolated and must evaluate to a scalar; placeholders are not supported in mapping keys. This is why gpol has a
`template/` package instead of `autogen/` — its generation mechanism is fundamentally different from vpol/mpol's.

## GlobalContextEntry projection expressions get their own small env, not one of the five kinds' envs

`pkg/cel/compiler/globalcontext_projection.go`'s `CompileGlobalContextProjection` builds a once-built, package-level
`cel.Env` scoped to `DefaultEnvOptions()` plus a single `object` variable — no `http`, `globalContext`, or other
Kyverno CEL libraries are registered, so a projection expression can read the value `globalContext.Get(name)`
returned (via `object`) but can't make further API calls or re-enter global context from inside a projection. This
is additive alongside the pre-existing `jmesPath` field on `GlobalContextEntryProjection`
(`api/kyverno/v2`/`v2alpha1`/`v2beta1`): a projection accepts exactly one of the two, enforced both by a
`+kubebuilder:validation:XValidation` marker on the type and by `GlobalContextEntryProjection.Validate()`.
`pkg/validation/globalcontext/validate.go` is the only webhook-side CEL compile check, since CEL compilation can't
live in `api/` (it already imports `api/kyverno/v2beta1`).

## Variables and match conditions compile in a read-only env

Every policy compiler builds a read/write env for the policy's actions and a read-only env (`readOnly=true` on the
sdk `resource`/`http` libs, which drops `post`/`Post`) for variables, match conditions, mpol target matching and
PolicyException match conditions. `compiler.NewWithVariablesProvider` points the read/write env's
`VariablesProvider` at the read-only env so variables are compiled once and their types are visible to both.
`pkg/cel/compiler/readonly.go` turns a `post` call in those fields into a `Forbidden` error instead of the generic
CEL one.

## PolicyException CEL match-conditions compile under one env at admission and execution

`pkg/cel/compiler/policy_exception.go`'s `CompilePolicyExceptionMatchConditions` caps `matchConditions` at 64
entries and compiles every expression against the caller-supplied env. The celexception webhook passes
`vpol/compiler.NewExceptionEnv(ns, true)`, a read-only `StoredExpressions` env. An exception's compensating-control
`validations` are actions, not matching, so they compile against `NewExceptionEnv(ns, false)` and may call `post`. There is no grandfathering of previously
stored expressions anymore: an update re-validates all of them under the same env.

## `ExceptionScope` is the only exception matcher

`pkg/cel/engine.ExceptionScope` (`exception.go`) is the sole namespace-matching logic for
`PolicyException` lookups across every policy kind's engine — don't write a second matcher. It
derives from the owning controller's `--exceptionNamespace` flag (a per-binary flag that can differ between
the admission, background, and reports controllers): unset (`""`) accepts no
exceptions, a concrete namespace accepts exceptions from itself only, and `"*"` accepts
exceptions from every namespace. For a namespaced policy specifically, that acceptance is further
narrowed: the exception must additionally come from the policy's own namespace, or from the scope's
configured central namespace when one is set — a namespaced policy under `"*"` never accepts an
exception from an arbitrary third namespace, only its own.

## Recent activity worth knowing about

CEL engine dependency itself has had a security fix (`2e9ef352e`, bumping `cel-go` for `GHSA-gcjh-h69q-9w9g` — the
GitHub Security Advisory's canonical identifier; it has no assigned CVE, so don't prefix it with `CVE-`).
`PolicyExceptions` are read from the manager cache, not a second informer (`47f45aebe`) — don't reintroduce a
second informer for exceptions. `globalContext` is explicitly denied in namespaced policies (`795394e1c`, see
above). Namespace scoping/routing for the newer namespaced CEL policy variants has had several recent bugs — see
[pkg/webhooks/AGENTS.md](../webhooks/AGENTS.md).
