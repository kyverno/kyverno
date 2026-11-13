# Meta

- Name: Rust SDK for local ValidatingPolicy evaluation
- Start Date: 2026-11-07
- Last Updated: 2026-11-08
- Status: Draft
- Author(s): @JimBugwadia, with Copilot-assisted research and drafting
- Supersedes: N/A
- Superseded By: N/A

# Table of Contents

- [Meta](#meta)
- [Table of Contents](#table-of-contents)
- [Overview](#overview)
- [Definitions](#definitions)
- [Motivation](#motivation)
- [Proposal](#proposal)
- [Implementation](#implementation)
- [Migration (OPTIONAL)](#migration-optional)
- [Drawbacks](#drawbacks)
- [Alternatives](#alternatives)
- [Prior Art](#prior-art)
- [Unresolved Questions](#unresolved-questions)
- [CRD Changes (OPTIONAL)](#crd-changes-optional)

# Overview

Build an embeddable Rust SDK that applies Kyverno ValidatingPolicy (VPOL) to
local JSON or JSON-compatible YAML payloads, with locally supplied exceptions
and portable evaluation reports. Deliver a thin HTTP service using the same
library, then expand deterministic CEL support through compatibility-tested
increments. Start with a bounded JSON-mode profile rather than promise full
VPOL parity. This is a research proposal; no runtime prototype has been validated.

This draft is stored here as requested. The submission venue remains governed
by the [proposal lifecycle](./README.md); this document does not change that process.

# Definitions

- **VPOL:** the `policies.kyverno.io` ValidatingPolicy API, whose definitions live
  in the separate [kyverno/api](https://github.com/kyverno/api) module.
- **CEL:** Common Expression Language, used for conditions and validations.
- **Capability profile:** a versioned declaration of supported policy fields,
  CEL features, overloads, and execution limits.
- **Local processing:** evaluation using only supplied policies, exceptions,
  payloads, and evaluation options, without cluster or external-service access.
- **Outcome:** whether evaluation passed, failed, errored, or skipped.
- **Decision:** the enforcement interpretation of outcomes and configured actions.
- **GC:** GlobalContext, a deferred external-data capability.

# Motivation

Applications should be able to reuse VPOL policy definitions without running a
Kubernetes cluster or embedding the Go controller. A Rust SDK supports native
application integration, while a service exposes the same evaluation semantics
to applications in other languages.

The first useful product must include policy lifecycle behavior, exceptions,
and reports, not merely a generic CEL interpreter. An explicit compatibility
profile makes a small implementation usable without silently accepting policies
whose intended behavior it cannot enforce.

# Proposal

Provide:

- `kyverno-vpol`: an embeddable library for compilation, bounded evaluation,
  exceptions, decisions, diagnostics, and structured reports.
- `kyverno-vpol-service`: an HTTP wrapper over that same library.

The compatibility promise is:

> Supported policies evaluate compatibly with a pinned Kyverno implementation.
> Unsupported behavior is rejected explicitly, never silently omitted.

For example, a locally supplied policy can validate application configuration:

```yaml
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: require-owner
spec:
  evaluation:
    mode: JSON
  validations:
    - expression: "has(object.owner) && object.owner != ''"
      message: An owner is required.
```

The payload `{"owner":"platform"}` passes; `{"owner":""}` fails. These are
intended examples for implementation fixtures, not evidence of a working SDK.

An illustrative embedded interface is:

```rust
let bundle = Engine::new(Profile::JsonBasic)
    .compile(policies, trusted_exceptions)?;

let result = bundle.evaluate(payload, options)?;
```

Compilation rejects unsupported behavior with a policy identity, field or
expression location, and capability diagnostic, for example:

```text
policy require-owner: unsupported evaluation mode Kubernetes; JsonBasic requires JSON
```

Results distinguish outcome from enforcement decision and include policy
identity, bundle revision, failing validation index, messages, warnings, applied
and refused exceptions, and the compatibility profile. An Audit failure remains
a failed evaluation even when the decision allows processing.

Exceptions come from a trusted configuration channel, not arbitrary evaluation
callers. Reports describe local payloads using caller-supplied identity; they do
not require Kubernetes UIDs or imply cluster report persistence.

## Scope

All delivery phases focus on local JSON processing. YAML is an input format
converted to JSON-compatible values.

Kubernetes clients, lookups, discovery, resource matching, namespace integration,
admission transport, autogen, native VAP generation, controllers, background
scanning, and cluster report lifecycle are excluded from the roadmap.

Outbound HTTP, GlobalContext, registry access, resource writes, and other advanced
capabilities are deferred. MPOL and IVPOL are separate potential future proposals.
They must not impose clients, credentials, or provider abstractions on this SDK.

The web service provides inbound HTTP transport for local evaluation. It does
not enable outbound HTTP CEL functions or require an external data source.

# Implementation

## Architecture

```text
Local policies + trusted local exceptions
                  |
                  v
Strict schema/profile validation -> CEL compilation -> immutable bundle
                                                            |
Local JSON/YAML payload -> bounded evaluation ---------------+
                                |
                                v
                    outcomes + decisions + reports
                                ^
                                |
                      Embedded API / HTTP service
```

Compiled bundles are immutable and reusable. Each evaluation owns its activation,
lazy-variable state, clock, diagnostics, and budget. Keep the CEL runtime behind
a narrow internal boundary; avoid a general plugin framework.

### Runtime recommendation

Provisionally prefer [cel-cxx](https://github.com/xjasonli/cel-cxx) because its
compile-time checker may offer a shorter path to typed VPOL expressions. This
produces a Rust SDK with a C++ dependency, not an entirely pure-Rust dependency
tree.

[cel-rust](https://github.com/cel-rust/cel-rust) is the pure-Rust alternative.
The research did not establish equivalent static checking or execution-budget
controls. Phase 0 must qualify these capabilities instead of assuming parity.

If neither candidate provides enforceable execution bounds, narrow the accepted
profile or address the runtime gap before release. An HTTP timeout alone does
not cancel synchronous CEL evaluation.

## Phase 0 - Qualify the CEL foundation

Deliver small experiments, not a full engine:

- Pin the Kyverno evaluator, policy API, and CEL-runtime compatibility baseline.
- Verify basic expressions, Boolean/string output checks, and scoped declarations.
- Prove lazy variables, CEL error propagation, and first-failure behavior.
- Verify concurrent program reuse without activation-state leakage.
- Establish compilation and evaluation bounds, including comprehensions.
- Verify reproducible builds on selected platforms.
- Establish differential fixtures against Kyverno's actual VPOL evaluator.

**Exit:** a selected runtime demonstrates the initial profile's semantics,
concurrency, and bounded execution. Record whether native C++ dependencies are
accepted. Resolve these gates before building the full SDK.

## Phase 1 - Working local SDK

Implement an explicit `JsonBasic` capability profile:

- `policies.kyverno.io/v1beta1` ValidatingPolicy with explicit JSON evaluation mode.
- Local JSON and JSON-compatible YAML payloads.
- CEL literals, field/index access, comparisons, Boolean logic, ternary
  expressions, bounded arithmetic, and basic string functions.
- Standard macros such as `has`, `all`, `exists`, `exists_one`, `map`, and
  `filter`, subject to Phase 0's tested bounds.
- Ordered match conditions, lazy variables, validations, and message expressions.
- Local CEL PolicyExceptions: references, match conditions, compensating
  controls, full exemptions, partial allowed values/images, report-result
  selection, priority, and expiry.
- Deny/Audit/Warn interpretation and failure-policy handling.
- Structured reports, local payload identity, and exception evidence.
- Actionable capability and compilation diagnostics.

Reject Kubernetes mode, unsupported spec behavior, unavailable variables, and
unimplemented CEL functions or overloads before evaluation, including references
in unreachable branches. Do not reinterpret the API's omitted evaluation-mode
default of Kubernetes as JSON. Reject unsupported JSON-profile fields, such as
audit annotations, rather than silently ignoring them.

**Exit:** supported differential fixtures have no unexplained semantic
differences; unsupported behavior rejects during compilation; equivalent
JSON/YAML inputs agree; exception, action, message-error, numeric-boundary,
concurrency, and resource-limit tests pass.

## Phase 2 - Local web service

Wrap the same SDK without introducing external data dependencies:

- Evaluation endpoint accepting local JSON/YAML payloads.
- Trusted configuration path for policies and exceptions.
- Atomic bundle replacement that preserves the last valid bundle on failure.
- Authentication, bounded workers and queues, request-size limits, and backpressure.
- Stable serialized results and actionable errors.
- Request-local state and explicit cancellation/limit behavior.

Configuration authority is separate from permission to submit payloads.

**Exit:** embedded and service results match; concurrent requests and reloads
remain isolated; overload and timeout behavior is tested; timed-out requests
cannot leave unbounded evaluation work running.

## Phase 3 - Broader local CEL compatibility

Expand deterministic libraries in demand-driven groups:

- Optional values, collections, strings, regex, lists, and sets.
- Hashing and bounded JSON/YAML parsing.
- Quantity, semver, IP/CIDR, and URL processing.
- Image-reference parsing only, without registry access or verification.
- Other local transformations after individual qualification.

**Exit per group:** supported overloads, values, errors, and resource bounds match
the pinned target. Publish the exact supported surface; a subset must not imply
complete support for a library.

HTTP/GC and other advanced capabilities are not scheduled phases. Revisit them
separately after the local evaluator is usable. Kubernetes integrations are
removed from the roadmap, not retained as committed later milestones.

## Evaluation semantics

Preserve Kyverno behavior within the supported profile:

- Evaluate exceptions before policy match conditions.
- Keep variables lazy; unused erroneous variables must not fail eagerly.
- Stop validations at the first false result or error and retain its index.
- Partial exceptions expose allowed values/images; they do not automatically
  exempt matching payloads.
- A refused exception does not deny an otherwise compliant payload.
- Distinguish false validations from evaluation errors when applying actions and
  failurePolicy. Ignore does not waive an ordinary false validation.
- Preserve a failed validation when its message expression fails.

Check exception expiry against an injectable evaluation clock. Document this
standalone choice because Kyverno's static and Kubernetes-backed exception
providers differ in expiry filtering.

Reports are portable projections, not replicas of cluster report lifecycle.
A skipped policy is not proof that its validations passed.

## Input and execution limits

- Bound payload, policy, and exception bytes, nesting, and collection sizes.
- Reject duplicate keys, non-string YAML mapping keys, and non-JSON YAML values.
- Preserve supported numeric values; reject unsupported ranges rather than round.
- Bound expression and expanded AST sizes.
- Bound compilation separately from evaluation.
- Apply an aggregate budget across exceptions, conditions, variables,
  validations, and messages.
- Accept only operations whose work and output can be bounded.
- Exclude arbitrary caller-defined functions from the initial profile.

Capability/configuration rejection remains fatal regardless of failurePolicy.
Budget exhaustion is visible as an evaluation error, never a successful pass.

## Verification

Use the pinned Kyverno VPOL compiler/evaluator as the differential oracle, not
merely standalone cel-go expression evaluation. Compare compilation acceptance,
outcome, validation index, actions, exceptions, and reports. Normalize timestamps
and runtime-specific wording, not semantic differences.

Prioritize missing/null fields, numeric boundaries, logical error absorption,
unused erroneous variables, first-failure ordering, partial/refused exceptions,
priority and expiry boundaries, failurePolicy/actions, message errors, YAML
equivalence, concurrent evaluations, bundle reloads, and budget exhaustion.

Supplement with relevant [official CEL conformance tests](https://github.com/cel-expr/cel-spec)
and explicit unsupported/known-difference lists.

### Research baseline

Research used Kyverno commit `da65b9ae2dbdddb7e51445cee22c672a01e8930b`.
Reconfirm versions during Phase 0. Relevant sources at that baseline:

- `pkg/cel/policies/vpol/compiler/compiler.go:68-215`: compilation modes.
- `pkg/cel/policies/vpol/compiler/policy.go:102-311`: ordered evaluation.
- `pkg/cel/policies/vpol/engine/engine.go:159-287`: exception and engine behavior.
- `pkg/cel/policies/vpol/engine/provider.go:43-56`: static exception association.
- `pkg/cel/engine/exception.go:67-86`: expiry filtering.
- `pkg/webhooks/resource/vpol/handler.go:164-201`: action/failure-policy reference
  semantics only; webhook integration remains out of scope.
- `pkg/utils/report/results.go:96-124,177-185,215-230`: report projection.
- `pkg/cel/compiler/cost.go:23-34` and
  `pkg/cel/requestbudget/requestbudget.go:18-35`: budget mechanisms.

## Link to the Implementation PR

N/A

# Migration (OPTIONAL)

No existing Kyverno behavior changes. Users opt into a separate SDK or service.
Only explicitly supported JSON-mode policies are accepted. Applications using
unsupported features receive compilation errors, not degraded enforcement.

Publish a capability matrix, examples, and adoption documentation with the
initial implementation. Pin compatibility versions so upgrades are reviewable.

# Drawbacks

- A second implementation can drift from Go CEL and VPOL semantics.
- The initial profile intentionally rejects many existing VPOL expressions.
- CEL C++ adds native build, packaging, and FFI maintenance costs.
- Pure-Rust alternatives may require additional checker or budgeting work.
- Exception and report parity adds work beyond expression evaluation.
- Resource bounds can constrain otherwise valid policies.
- No benchmark or prototype currently establishes performance or runtime suitability.

# Alternatives

- **Pure Rust from the outset:** prefer if purity is required, but qualify
  checking and budgets before selecting the runtime.
- **Wrap the Go evaluator:** reduces semantic duplication but introduces Go
  runtime/FFI or process-management dependencies for embedded applications.
- **Service-only Go deployment:** reuses existing evaluation but does not meet
  the embedded Rust use case.
- **Full VPOL parity initially:** substantially increases scope and conflicts
  with delivering a usable local evaluator first.
- **No SDK:** applications continue using existing Go integrations or services;
  native Rust embedding remains unaddressed.

# Prior Art

- [Kyverno SDK](https://github.com/kyverno/sdk): existing ecosystem libraries.
- [Kyverno JSON](https://github.com/kyverno/kyverno-json): adjacent local-data
  policy-engine work; this proposal specifically targets VPOL semantics.
- [cel-cxx](https://github.com/xjasonli/cel-cxx): Rust bindings to CEL C++.
- [cel-rust](https://github.com/cel-rust/cel-rust): pure-Rust CEL implementation.
- [CEL specification](https://github.com/cel-expr/cel-spec): language semantics
  and conformance fixtures.

# Unresolved Questions

Before acceptance:

- Confirm whether C++ dependencies are acceptable or pure Rust is required.
- Select the runtime using Phase 0 evidence for correctness and resource bounds.
- Select supported platforms, Rust MSRV, and the initial pinned compatibility target.
- Confirm crate/repository ownership and distribution strategy.

During implementation:

- Finalize API names, serialized report schema, and numeric representation.
- Select concrete resource-limit defaults using qualification results.
- Define service endpoint details and deployment-specific authentication.
- Prioritize Phase 3 library groups based on real policies.

Intentionally deferred: outbound HTTP, GlobalContext, other advanced
capabilities, MPOL, and IVPOL. Kubernetes integrations are outside this roadmap.

# CRD Changes (OPTIONAL)

None. Consume the external policy API without changing its CRDs. Rust models
and supported-field validation are versioned SDK concerns, not a new Kubernetes API.
