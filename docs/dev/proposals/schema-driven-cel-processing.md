# Meta
[meta]: #meta

- Name: Schema-Driven CEL Processing
- Start Date: 2026-09-20
- Last Updated: 2026-09-22
- Status: Proposed
- Author(s): [@JimBugwadia](https://github.com/JimBugwadia)
- Supersedes: N/A

# Table of Contents
[table-of-contents]: #table-of-contents

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
[overview]: #overview

Kyverno should support policy evaluation over externally supplied structured
payloads without requiring a new Go type library, compiler branch, evaluation
mode, and server for every integration. This proposal introduces reusable
`PayloadSchema` and `PayloadBinding` resources, one schema-driven policy mode,
and typed CEL activation generated from either a restricted JSON Schema dialect
or protobuf descriptors. It preserves explicit transport adapters where wire
protocol behavior requires them while making new payload contracts primarily a
configuration and policy concern.

## Summary

Introduce a schema-driven evaluation contract, not a plugin mechanism:

1. A cluster-scoped **`PayloadSchema`**, initially
   `policies.kyverno.io/v1alpha1`, describes JSON validation, schema selection
   hints, the fields exposed to CEL, and reusable computed values.
2. An operator-owned **`PayloadBinding`** associates an authenticated ingress
   route with an explicit schema or a bounded set of candidate schemas, required
   policies, resource limits, and an optional JSON response projection.
3. Add one reusable **`Schema`** evaluation mode and a named reference:
   `spec.evaluation: {mode: Schema, schemaRef: {name: nono-approval-v1}}`.
   Preserve all existing mode strings. Never add another mode for a payload shape.
4. Validate the original JSON, then expose a projected **`object`** through
   schema-generated CEL type declarations and request-scoped value adapters.
   This preserves static field checking without generating Go structs.
5. Decode once, compile off the request path, and lazily adapt nested values and
   computed fields. `Activation.ResolveName` alone does **not** make nested JSON
   lazy. Full JSON validation still examines the original document.
6. Prefer explicit routing. On shared routes, use cheap discriminator/shape
   predicates followed by CEL conditions; exactly one schema must match.
   Ambiguity, missing enforcement dependencies, and invalid input fail closed.
7. Keep policy validations boolean. Aggregate decisions generically; translate
   the final decision to a JSON response declaratively. Retain protocol adapters
   for actual Envoy gRPC/HTTP ext_authz transport.
8. **Plugin mechanisms (in-process Go plugins, WASM, or an external
   decoder/processor service) were evaluated and rejected for this release**
   ([Alternatives](#alternatives)): they relocate the authoring burden instead
   of removing it, run untrusted/uncheckable code on the admission or
   authorization path, and reintroduce per-integration result types by
   construction. A schema is data the existing CEL compiler can validate
   statically; a plugin is code it cannot.
9. **A second, optional `ProtobufDescriptorV1` dialect is proposed**
   ([protobuf dialect](#optional-dialect-protobufdescriptorv1-for-grpc-payloads))
   alongside JSON Schema, because Envoy's real `ext_authz` gRPC service is
   protobuf-native, while nono.sh's real webhook contract is JSON/HTTP-native
   ([external evidence](#external-evidence-and-research-limitations)). One
   dialect does not fit both, so the design adds a second dialect rather than
   forcing a JSON encoding of protobuf semantics.

The result is **configuration-only onboarding for new JSON or protobuf
contracts over supported transports**. It is not a claim that a schema can
implement a new network protocol, authenticate a new issuer, or reproduce
arbitrary Go methods.

All APIs, fields, paths marked “new,” defaults, and limits below are proposals.
YAML is illustrative future configuration, not configuration accepted by the
current binary.

# Definitions
[definitions]: #definitions

- **Payload**: The structured request value evaluated by a policy. It may be a
  JSON document or a protobuf message.
- **PayloadSchema**: A proposed cluster-scoped resource defining the payload
  dialect, type contract, CEL-visible projection, selection hints, and computed
  fields.
- **PayloadBinding**: A proposed operator-owned resource binding an
  authenticated ingress route or gRPC method to candidate schemas, policies,
  limits, and a response projection.
- **Schema mode**: The proposed reusable `ValidatingPolicy` evaluation mode
  that references a `PayloadSchema`; it replaces per-integration mode constants.
- **Projection**: The subset of a validated payload exposed to CEL after
  applying include and exclude paths.
- **Computed field**: A schema-owned CEL expression evaluated lazily and
  exposed through `derived` for reuse by policies.
- **Dialect**: The schema language used to type and validate a payload. The
  initial proposal covers a restricted JSON Schema 2020-12 profile and an
  optional protobuf `FileDescriptorSet` profile.
- **Activation**: The request-scoped values supplied when a compiled CEL
  program is evaluated, including `object`, `derived`, policy variables, and
  transport metadata.
- **Fail closed**: Denying a request when selection, validation, compilation,
  or evaluation cannot produce an authoritative allow decision.

# Motivation
[motivation]: #motivation

## Problem Statement and Verified Baseline

### The repeating integration pattern

An external authorization integration currently chooses a payload model,
registers CEL types/functions, selects an evaluation mode, compiles policies for
that environment, and exposes a server. The schema, CEL bindings, routing,
failure handling, exception support, and observability become tightly coupled.

`kyverno-authz` demonstrates this pattern. The proposed kyverno-nono integration
would repeat it for agent command/endpoint approvals. Repeating the process for
every SaaS webhook or agent protocol multiplies binaries, compiler branches,
release coordination, and independently maintained enforcement semantics.

The underlying problem is not that CEL cannot evaluate JSON. It already can.
The missing capability is a **first-class, validated, typed, routable external
payload contract** shared by compilers, policy authors, ingress, and operators.

### Corrections to assumptions in the motivating examples

Research found several distinctions important to implementation:

* **JSON mode already exists.** The local validating-policy compiler dispatches
  JSON separately and otherwise compiles Kubernetes mode. Both environments
  declare `object` as `cel.DynType`. JSON evaluation accepts an arbitrary value
  and binds it to `object`; no integration-specific struct is necessary for
  basic expressions.
* **Modes are not currently a CRD-enforced enum.** The upstream API defines
  `EvaluationMode = string`; the generated local CRD declares `mode` as a string,
  with descriptive text listing Kubernetes, HTTP, and Envoy, but no `enum`.
  Dispatch and constants nevertheless make supported behavior fixed in code.
  Arbitrary mode strings do not provide schema-driven execution.
* **The CEL policy Go API is external to this checkout.** Imports reference
  `github.com/kyverno/api/api/policies.kyverno.io/...`. Local `api/kyverno/...`
  contains legacy Kyverno APIs, not the authoritative `ValidatingPolicy` type.
  `v1` aliases `v1beta1`, which aliases `v1alpha1` specification types at the
  pinned API revision. An upstream API change and dependency update are required.
* **Authz has two distinct type models.** HTTP mode registers native structs
  with `cel` tags, while Envoy mode registers the actual Envoy protobuf messages
  using `cel.Types`. The HTTP wrapper's `attributes.method`, `header`, and `path`
  are not the Envoy `attributes.request.http` protobuf shape.
* **Authorization result semantics differ.** Authz compiles validations returning
  a typed CheckResponse or null. The local validating-policy compiler requires
  boolean validation results. Migration must translate decision semantics, not
  just rename an input type.
* **The nono prototype confirms, in working code, the exact pattern this
  design replaces.** Branch `kyverno-nono-design`
  (`461922d1ca5c87110ef70f0e7e6e1c764095529b`, worktree
  `kyverno-nono-worktree`) adds a hand-written `nono.RequestData` struct with a
  flattened union of command and endpoint fields (populated/zero-valued
  depending on `capability_type`), registers it via `ext.NativeTypes(...,
  ext.ParseStructTags(true))` — the identical technique authz's HTTP mode
  uses — adds `nono.Grant()`/`nono.Deny(reason)`/`.Response()`/`object.request.argv()`
  as hand-written CEL overloads, defines a new mode constant
  `EvaluationModeNono vpol.EvaluationMode = "Nono"`, and ships a full parallel
  compiler (`pkg/nono/compiler`) that duplicates authz's compiler structure
  (base env, lazy `variables` provider from
  `k8s.io/apiserver/pkg/cel/lazy.NewMapValue`, match conditions, exceptions,
  first-non-null rule evaluation) parameterized on the new types instead of
  authz's. It also ships its own binary (`cmd/kyverno-nono`) with its own
  fail-closed `/approve` handler, and depends on external, unreleased
  `github.com/kyverno/kyverno-authz` and `github.com/kyverno/sdk` packages for
  the engine, sources, and events/metrics plumbing it reuses. This is real,
  running evidence — not a hypothetical — that every new payload shape today
  costs a new struct, a new CEL library, a new mode constant, a new compiler,
  and a new server, even when 90% of the compiler and server logic (match
  conditions, exceptions, lazy variables, fail-open/closed dispatch, events)
  is copy-pasted unchanged from the previous integration.
* A branch named `kyverno-nono-prototype` was also inspected locally; it
  contains no nono-specific commits (it is a plain, older snapshot of upstream
  `kyverno/kyverno` main, 156 commits behind the current baseline, with an
  empty reflog). It is not a source of implementation evidence for this
  design; `kyverno-nono-design` is the only branch with nono code.

### Local source evidence

Line references in this document are to the baseline above unless marked
external. They describe current behavior, not proposed interfaces.

| Evidence | Source |
|---|---|
| Pinned API, authz, and CEL dependencies | `go.mod:35-37,253` (`kyverno/api` at `fb2785727f98`, authz at `9101a3ffd44f`, cel-go `v0.31.0`) |
| Compiler mode dispatch | `pkg/cel/policies/vpol/compiler/compiler.go:55-62` |
| JSON compilation and dynamic `object` environment | `pkg/cel/policies/vpol/compiler/compiler.go:138-234` |
| JSON activation; exceptions; match conditions; lazy policy variables | `pkg/cel/policies/vpol/compiler/policy.go:54-63,80-155` |
| Match-condition false/error/Ignore behavior | `pkg/cel/policies/vpol/compiler/policy.go:201-234` |
| Variables type provider and inferred field types | `pkg/cel/compiler/variables.go:8-67`; `pkg/cel/compiler/compiler.go:49-78` |
| Boolean validation result requirement | `pkg/cel/compiler/compiler.go:136-151` |
| Existing optional types and compatibility behavior | `pkg/cel/compiler/env.go:56-65,98-154` |
| OpenAPI → CEL declaration precedent | `pkg/cel/resource/openapi.go:20-26` |
| Existing JSON request carrier | `pkg/cel/engine/request.go:14-31` |
| JSON engine branch iterates the fetched policies before the later predicate check | `pkg/cel/policies/vpol/engine/engine.go:50-65,98-101` |
| Policy/exception watches and autogeneration | `pkg/cel/policies/vpol/engine/provider.go:30-78,84-178` |
| Current compile-error handling and flattened policy cache | `pkg/cel/policies/vpol/engine/reconciler.go:87-99,119-132` |
| Current policy validation requires Kubernetes resource rules | `pkg/cel/policies/vpol/validate.go:26-34` |
| CEL validation webhook entry point | `pkg/webhooks/policy/handlers.go:47-53` |
| Pod-oriented CEL autogeneration | `pkg/cel/policies/vpol/autogen/autogen.go:19-33,45-55` |
| Legacy engine versus CEL engine boundary | `pkg/engine/engine.go:8-20,27-36`; `pkg/cel/policies/vpol/engine/engine.go:9-19` |
| Generated evaluation/failure policy schema | `config/crds/policies.kyverno.io/policies.kyverno.io_validatingpolicies.yaml:129-177` |
| API source used by CRD generation | `Makefile:580-588` |
| New-kind, additive-field, and version-reference rules | `docs/dev/api/README.md:53-76` |
| nono flattened union struct, zero-valued by capability | `pkg/cel/libs/authz/nono/types.go:29-48` (branch `kyverno-nono-design`) |
| nono native-type registration, identical technique to authz HTTP mode | `pkg/cel/libs/authz/nono/lib.go:24-37` |
| nono hand-written `Grant`/`Deny`/`Response`/`argv` CEL overloads | `pkg/cel/libs/authz/nono/lib.go:41-96`, `pkg/cel/libs/authz/nono/impl.go:41-52` |
| nono per-integration mode constant | `pkg/cel/libs/authz/nono/constants.go:5-8` |
| nono compiler duplicating authz's base env / lazy variables / exceptions / first-non-null dispatch | `pkg/nono/compiler/compiler.go:1-29,56-79,197-260` |
| nono fail-closed `/approve` handler (denies on no match, unlike authz's default-allow) | `cmd/kyverno-nono/handlers/approve.go:66-108` |
| nono server wiring depends on external, unreleased authz/sdk packages | `cmd/kyverno-nono/internal/serve.go:1-45` |

The existing JSON engine branch must not be reused by merely supplying a
predicate: that branch currently does not apply the later predicate. Selection
must occur explicitly in a schema-indexed provider/engine path.

### External evidence and research limitations

Pinned authz sources:

* [Native HTTP types, lines 26–61](https://github.com/kyverno/kyverno-authz/blob/9101a3ffd44f/pkg/cel/libs/authz/http/types.go#L26-L61)
  and [native registration, lines 24–35](https://github.com/kyverno/kyverno-authz/blob/9101a3ffd44f/pkg/cel/libs/authz/http/lib.go#L24-L35).
* [Envoy protobuf registration, lines 12–43](https://github.com/kyverno/kyverno-authz/blob/9101a3ffd44f/pkg/cel/libs/authz/envoy/lib.go#L12-L43)
  and [response helpers, lines 59–100](https://github.com/kyverno/kyverno-authz/blob/9101a3ffd44f/pkg/cel/libs/authz/envoy/lib.go#L59-L100).
* [Mode-specific environments, lines 53–82](https://github.com/kyverno/kyverno-authz/blob/9101a3ffd44f/pkg/cel/env.go#L53-L82);
  [input and result typing, lines 64–85 and 158–182](https://github.com/kyverno/kyverno-authz/blob/9101a3ffd44f/pkg/engine/compiler/compiler.go#L64-L85);
  [mode constants](https://github.com/kyverno/kyverno-authz/blob/9101a3ffd44f/apis/constants.go#L7-L10).
* Authz's `pkg/events/` contains `iface.go`, `k8sevents.go`, `openreports.go`,
  `ringbuf.go`, and `writer.go` at this revision. Reuse observability contracts;
  do not copy this package wholesale for each future payload.
* **Authz's Envoy integration is a real gRPC service, not a JSON adapter.**
  [`pkg/authz/envoy/server.go`](https://github.com/kyverno/kyverno-authz/blob/ea97befa33fdac0a7138ab639f7e26c00e404321/pkg/authz/envoy/server.go)
  constructs a `grpc.NewServer()` and calls
  `authv3.RegisterAuthorizationServer(s, svc)` against the generated
  `envoyproxy/go-control-plane` Envoy protobuf stubs;
  [`pkg/server/grpc.go`](https://github.com/kyverno/kyverno-authz/blob/main/pkg/server/grpc.go)
  runs it with graceful shutdown. CEL policies evaluate directly against the
  real `authv3.CheckRequest` protobuf message ([baseline corrections](#corrections-to-assumptions-in-the-motivating-examples), `envoy/lib.go:12-43`).
  This confirms Envoy's `ext_authz` filter (see the
  [Envoy ext_authz documentation](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter))
  is gRPC/protobuf end to end, motivating the [protobuf dialect proposal](#optional-dialect-protobufdescriptorv1-for-grpc-payloads).
* **nono.sh is a real, independent project**, not a synthetic example:
  [nono.sh](https://nono.sh/) and
  [`nolabs-ai/tool-sandbox-examples`](https://github.com/nolabs-ai/tool-sandbox-examples)
  describe a kernel-level (Landlock/Seatbelt) sandbox for AI-agent tool
  execution, with a Kubernetes demo that calls a local
  `approval-webhook-demo.py` over HTTP/JSON for commands/endpoints outside
  static policy — the same request/response shape the kyverno-nono prototype
  ([nono example](#nono-approval-verified-contract-from-the-working-prototype)) reimplements as a Kyverno approval server. nono's wire protocol is
  HTTP/JSON, not gRPC, which is why the [protobuf dialect](#optional-dialect-protobufdescriptorv1-for-grpc-payloads) treats gRPC support as an additional,
  independent dialect rather than something nono itself requires.

Pinned API sources:

* [Effective evaluation configuration](https://github.com/kyverno/api/blob/fb2785727f98/api/policies.kyverno.io/v1alpha1/common.go#L3-L19).
* [Effective ValidatingPolicy specification](https://github.com/kyverno/api/blob/fb2785727f98/api/policies.kyverno.io/v1alpha1/validating_policy.go#L55-L121).
* [v1 aliases](https://github.com/kyverno/api/blob/fb2785727f98/api/policies.kyverno.io/v1/validating_policy.go#L8-L14)
  and [v1beta1 aliases](https://github.com/kyverno/api/blob/fb2785727f98/api/policies.kyverno.io/v1beta1/validating_policy.go#L10-L16).

**Update:** the nono design and prototype are now available locally
(worktree `kyverno-nono-worktree`, branch `kyverno-nono-design`, commit
`461922d1ca5c87110ef70f0e7e6e1c764095529b`, also pushed to
`origin/kyverno-nono-design` on `JimBugwadia/kyverno`) and were reviewed
directly — both the design docs
(`docs/dev/kyverno-nono/DESIGN.md`, `docs/dev/kyverno-nono/README.md`) and the
working prototype code (`pkg/cel/libs/authz/nono/`, `pkg/nono/compiler/`,
`cmd/kyverno-nono/`). A second branch, `kyverno-nono-prototype`, was checked
and found to contain no nono-specific commits (see [baseline corrections](#corrections-to-assumptions-in-the-motivating-examples)); it is not used as a
source. The wire protocol, Go model, `/approve` endpoint, fail-closed
semantics, and reporting-reuse pattern described in [baseline corrections](#corrections-to-assumptions-in-the-motivating-examples) above and in the
worked migration ([nono example](#nono-approval-verified-contract-from-the-working-prototype)) reflect this real prototype rather than an
illustrative contract. The nono wire fields (`capability_type`, `request_id`,
`session_id`, `child_pid`, `command`, `caller`, `args`, `intercept_rule`,
`method`, `path`, `route_id`, `upstream`, `rule_label`) come directly from
`pkg/cel/libs/authz/nono/types.go`.

## Goals and Non-Goals

### Goals

* Onboard a new supported JSON payload shape with CRDs and policies only.
* Validate payloads and catch misspelled fields/type errors when compiling
  policies, with errors pointing into both policy expressions and schemas.
* Support nested objects, typed dictionaries, lists, optional fields, bounded
  references and composition, and explicitly dynamic regions.
* Make schema choice deterministic, explainable, and resistant to bypass through
  an attacker-controlled discriminator.
* Bound hot-path CPU, allocations, memory, and CEL work.
* Preserve `ValidatingPolicy` variables, match conditions, failure actions, and
  `PolicyException` semantics where applicable.
* Share the same schema compiler and evaluation contract among server and CLI.

### Non-goals for the first release

* No arbitrary Go, JavaScript, WASM, shell, or remotely loaded function plugins.
* No claim to support all JSON Schema dialects or every 2020-12 keyword.
* No protocol generation from JSON Schema; no new gRPC service *invented* from
  a schema. ([the optional protobuf dialect](#optional-dialect-protobufdescriptorv1-for-grpc-payloads) only decodes/serves methods
  whose request/response types already exist in an operator-supplied
  descriptor set — it does not synthesize a new service definition.)
* No mutation/generation/image-verification schema mode in the first milestone.
  These need separate result and side-effect contracts.
* No Kubernetes admission autogen, background resource scans, or native
  ValidatingAdmissionPolicy generation for external payloads.
* No reconstruction of omitted payload branches for policies, schema defaults
  that silently change input, or inference that “no policy ran” means allow.
* No promise of exact authz header/query mutation parity in the first release.

# Proposal

## Proposed API

### Resources and ownership

**`PayloadSchema`** owns the reusable data contract and CEL view. Its `spec` is
immutable in the initial design: incompatible or compatible edits both create a
new revisioned resource name, such as `nono-approval-v2`. This makes review,
replay, and rollback straightforward while hot-swapping bindings remains
possible. Status and metadata can change.

**`PayloadBinding`** owns exposure: route, authenticated client access, candidate
schemas, required policy names, limits, and response formatting. Separating it
prevents a schema author from independently exposing a network endpoint or
weakening its enforcement policy. Both kinds are cluster-scoped and
operator-managed initially; namespace delegation is a later API decision.

The server's listener, TLS trust, client authentication, and authorization are
deployment configuration, not executable schema content. The listener maps a
validated mTLS or OIDC identity to a stable authenticated principal; it must
not derive identity from forwarded or caller-supplied headers. Each binding
contains an explicit authorization allowlist of those principals. Exact
method/path matching selects the binding first, then the server checks that the
principal is allowed for that binding before reading or selecting a schema.
A caller-supplied binding name, schema name, or discriminator is never a
permission grant. Authentication failure returns 401, an unknown
method/path returns 404, and a known route with a principal not allowed by its
binding returns 403; these outcomes must not fall back to another binding.

### PayloadSchema example

This complete example illustrates the proposed keyword profile, matching,
projection, local `$ref`, and reusable computed fields:

```yaml
apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadSchema
metadata:
  name: build-event-v1
spec:
  dialect: JSONSchema202012SubsetV1
  compatibilityVersion: "1"
  schema:
    $schema: https://json-schema.org/draft/2020-12/schema
    type: object
    additionalProperties: false
    required: [eventType, repository, artifacts]
    $defs:
      artifact:
        type: object
        additionalProperties: false
        required: [name, size]
        properties:
          name: {type: string, maxLength: 256}
          size: {type: integer, minimum: 0, maximum: 2147483647}
    properties:
      eventType: {type: string, const: build.completed}
      repository: {type: string, maxLength: 512}
      artifacts:
        type: array
        maxItems: 200
        items: {$ref: "#/$defs/artifact"}
      request:
        type: object
        additionalProperties: false
        properties:
          headers:
            type: object
            maxProperties: 100
            additionalProperties: {type: string, maxLength: 4096}
  match:
    transport:
      methods: [POST]
      paths: [/events]
      headers:
        x-event-family: [build]
    discriminators:
      - pointer: /eventType
        equals: build.completed
    requiredPaths: [/repository, /artifacts]
    conditions:
      - name: non-empty-repository
        expression: >-
          has(raw.repository) &&
          type(raw.repository) == string &&
          raw.repository != ""
  exposure:
    include: ["/**"]
    exclude: ["/request/headers/**"]
  computed:
    - name: artifactNames
      expression: object.artifacts.map(a, a.name)
```

`schema` is an embedded JSON document. Its CRD storage field must preserve
unknown schema keywords structurally; the PayloadSchema admission validator
then rejects unsupported assertion keywords with exact JSON Pointer errors.
Do not rely on API-server pruning to implement the schema dialect. An
`apiextensionsv1.JSON`-style field with appropriate generated structural schema
is preferable to pretending that CRD OpenAPI's `JSONSchemaProps` implements the
whole JSON Schema 2020-12 vocabulary.

The `$schema` URL identifies the underlying standard; `dialect` narrows it to
the documented supported profile. It does not cause network retrieval.
Known annotations (`title`, `description`, `$comment`, `examples`, `default`)
are accepted as documentation; `default` is not applied.

Suggested status:

```yaml
status:
  observedGeneration: 1
  digest: "sha256:<canonical-spec-digest>"
  conditions:
    - type: Ready
      status: "True"
      reason: Compiled
      message: Schema, selectors, exposure, and computed expressions compiled
```

The digest covers the complete immutable executable spec: schema, match rules,
exposure, computed fields, dialect, and compatibility version; it excludes
metadata and status. Status uses normal Kubernetes condition timestamps and
observed-generation fields in the implemented API.

### ValidatingPolicy reference shape

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: signed-build-artifacts
spec:
  evaluation:
    mode: Schema
    schemaRef:
      name: build-event-v1
      # Optional deployment pin; omitted here because no digest is computed yet.
      # digest: "sha256:<canonical-spec-digest>"
    admission:
      enabled: false
    background:
      enabled: false
  failurePolicy: Fail
  validationActions: [Deny]
  matchConditions:
    - name: production-repository
      expression: object.repository.startsWith("production/")
  variables:
    - name: names
      expression: derived.artifactNames
  validations:
    - expression: variables.names.all(n, n.endsWith(".signed"))
      message: Production artifacts must be signed
```

Exact validation rules:

* `mode: Schema` requires `schemaRef.name`; `schemaRef` is forbidden for all
  other modes. A schema name is not placed in `mode`.
* `schemaRef` refers only to the fixed group/kind `policies.kyverno.io /
  PayloadSchema`; no arbitrary GVK resolution. `digest`, if present, must match.
* `Schema` mode requires admission/background disabled; the schema-aware
  defaulting path sets them false when omitted. Explicit true is rejected.
  Examples specify false to avoid dependence on existing nested defaults.
* Kubernetes `matchConstraints`, Pod autogen, webhook timeout configuration,
  and VAP generation settings are forbidden here, not silently ignored.
  Transport limits belong to the binding.
* Initially, only cluster-scoped `ValidatingPolicy` participates. Reject
  `Schema` mode on `NamespacedValidatingPolicy` until namespace/tenant isolation
  is designed; do not accidentally flatten namespaced policies into this index.
* Variables remain policy-local and unavailable in policy match conditions.
  `derived` is schema-level and is available before those conditions.
* A schema reference resolving to a missing/not-ready resource is a policy
  readiness error. Syntax and locally detectable errors are admission errors.
  GitOps may create an unresolved reference, but a binding cannot activate it.

Choose `Schema` plus `schemaRef` rather than `mode: <schema-name>` because it
retains mode filtering, avoids name collisions, and makes the API additive. An
alternative `JSON` plus `schemaRef` saves one constant but makes old JSON
consumers more likely to evaluate a policy without its required validation.

### PayloadBinding: explicit and shared routes

Recommended explicit routing:

```yaml
apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadBinding
metadata:
  name: build-events
spec:
  route:
    path: /events
    methods: [POST]
  authorization:
    allowedPrincipals:
      - spiffe://example.com/build-system
  selection:
    schemaRef:
      name: build-event-v1
  requiredPolicies:
    - name: signed-build-artifacts
  enforcement:
    requireApplicablePolicy: true
  limits:
    maxBodyBytes: 1048576
    maxDepth: 64
    maxCandidates: 16
    maxCELCost: 100000
    timeoutMillis: 100
  response:
    format: Decision
```

For a shared route, replace `selection.schemaRef` with an exact bounded list:

```yaml
selection:
  candidates:
    - name: build-event-v1
    - name: deployment-event-v1
```

This is a configuration fragment; `deployment-event-v1` must be separately
defined and ready. A binding may use exactly one of `schemaRef` or `candidates`.
For multi-schema bindings, `requiredPolicies` must include at least one enforcing
policy for **each** candidate schema. Policy names are resolved to schema
identities while building the binding, not guessed at request time.

All active Schema-mode policies referencing the selected schema are evaluated;
`requiredPolicies` is a readiness/coverage floor, **not** an allowlist that can
exclude another enforcing policy. In alpha, different tenants should use
different schema resource identities even when their schema documents match.

Deleting a required policy, compiling it unsuccessfully, or removing its Deny
action makes the binding unready. Additional policies also appear as error
entries rather than disappearing when compilation fails. No route may start
without a required Deny policy. `requireApplicablePolicy` defaults true and
must remain true on authorization routes.

The `authorization.allowedPrincipals` entries are matched against the
principal produced by the deployment's authenticated listener. A binding is
not ready if its authorization configuration is empty or invalid. Route
registration is exact on canonical method/path pairs; conflicting
registrations are unready rather than resolved by order. The implementation
must include readiness tests for empty/invalid principal mappings and
conflicting routes, plus authenticated integration tests covering 401 for
authentication failure, 403 for a principal denied by an otherwise known
binding, 404 for an unknown route, and the absence of fallback to another
binding or schema.

### API versioning and source ownership

Create the new kinds in `policies.kyverno.io/v1alpha1`, not `kyverno.io/v1`.
Add `schemaRef` to the existing evaluation specification without changing any
existing field's representation or meaning. Use a small, version-neutral
reference structure (name/digest), **not** a stable Go API field whose type
imports an alpha `PayloadSchemaSpec`. Resource references do not require
embedding an alpha schema type in a stable specification.

The current alias chain already crosses versions; do not perpetuate it by
embedding new alpha resource structs into stable policy types. Review
additive-field compatibility, conversion, and schema preservation for every
served policy version in the upstream API repository. Feature-gate execution
and validate consumer compatibility before enabling it.

This follows the local versioning guidance at
`docs/dev/api/README.md:57-76`. No existing mode or field is removed; future
deprecation must follow the stable three-minor-release notice.

### Optional dialect: `ProtobufDescriptorV1` for gRPC payloads

JSON Schema is not the only structured contract worth generalizing. Both
motivating examples have a real gRPC-native side, confirmed in the actual
kyverno-authz repository (not hypothetical):

* Envoy's `ext_authz` filter calls a gRPC `Authorization` service defined by
  a protobuf `CheckRequest`/`CheckResponse` pair
  ([Envoy ext_authz filter docs](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter)).
  kyverno-authz runs a real `grpc.NewServer()`, registers
  `authv3.RegisterAuthorizationServer(s, svc)`, and evaluates CEL directly
  against the generated `authv3.CheckRequest` protobuf type — see
  [`pkg/authz/envoy/server.go`](https://github.com/kyverno/kyverno-authz/blob/ea97befa33fdac0a7138ab639f7e26c00e404321/pkg/authz/envoy/server.go)
  and [`pkg/server/grpc.go`](https://github.com/kyverno/kyverno-authz/blob/main/pkg/server/grpc.go).
  This is a genuine, running gRPC integration, distinct from authz's separate
  HTTP-native-struct mode ([Envoy example](#envoy-checkrequest-schema-and-equivalent-decision-policy)).
* nono.sh itself is a real project
  ([nono.sh](https://nono.sh/), [`nolabs-ai/tool-sandbox-examples`](https://github.com/nolabs-ai/tool-sandbox-examples)):
  its Kubernetes demo runs an AI agent in a kernel-isolated sandbox and calls a
  local `approval-webhook-demo.py` over HTTP/JSON when a command or endpoint
  falls outside static policy. The kyverno-nono prototype ([nono example](#nono-approval-verified-contract-from-the-working-prototype)) reimplements
  that same webhook contract; it does not need gRPC because nono's own wire
  protocol is HTTP/JSON, not protobuf.

Because one of the two motivating integrations (Envoy) is gRPC/protobuf-native
at the transport layer and the other (nono) is JSON/HTTP-native, a schema
mechanism that only understands JSON Schema cannot fully replace authz's Envoy
mode. Recommendation: add `dialect: ProtobufDescriptorV1` as a second
`PayloadSchema` dialect, reusing the same CRD, matching, exposure, and
computed-field mechanics as JSON Schema, but sourcing type information from a
compiled `google.protobuf.FileDescriptorSet` instead of a JSON Schema document.

```yaml
apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadSchema
metadata:
  name: envoy-authz-check-v1
spec:
  dialect: ProtobufDescriptorV1
  compatibilityVersion: "1"
  protobuf:
    # A base64 FileDescriptorSet (e.g. `buf build -o -` or `protoc
    # --descriptor_set_out`), or a ConfigMap/OCI reference for larger sets.
    descriptorSetRef:
      configMapRef: {name: envoy-authz-proto-fds, namespace: kyverno}
    messageType: envoy.service.auth.v3.CheckRequest
  match:
    requiredPaths: [/attributes/request/http/method]
  exposure:
    include:
      - /attributes/request/http/method
      - /attributes/request/http/path
      - /attributes/request/http/headers
    exclude:
      - /attributes/request/http/headers/authorization
```

**Why this is feasible without code generation per integration:** cel-go's
protobuf support is built on `google.golang.org/protobuf/reflect/protoreflect`,
not on generated Go structs specifically. A message built from a
`FileDescriptorSet` at runtime via `protodesc.NewFile` +
`dynamicpb.NewMessage` implements the same `protoreflect.Message` interface as
a `protoc`-generated type, and `cel.Types(...)` / the CEL protobuf type
provider work against that interface either way — this is the same technique
`grpcurl`, `protoreflect`-based gateways, and Envoy's own gRPC-JSON transcoder
use to handle arbitrary descriptors generically. The schema compiler would
load the descriptor set once, register it with a `protoregistry.Files`, and
build CEL field declarations from the message descriptor the same way [Type Mapping](#type-mapping)
builds them from JSON Schema — `oneof`, `repeated`, `map<>`, and well-known
wrapper types map onto CEL more directly than JSON Schema's `oneOf`/`anyOf` do,
because they are already part of cel-go's supported protobuf type system.

**On the transport side, this is deliberately narrower than the JSON case.**
Generically serving an arbitrary gRPC *service* (not just decoding a message)
requires either the operator's real generated stubs, or a dynamic dispatch
technique such as `grpc.UnknownServiceHandler` combined with
`dynamicpb.Message` decoding (the same mechanism gRPC reflection/proxy tools
use). The design should provide exactly one built-in dynamic gRPC front end
implementing this pattern for unary request/response methods described by the
same `FileDescriptorSet`, so a `PayloadBinding` can expose
`transport: gRPC` the same way it exposes `transport: HTTP` today. This is
narrower than full gRPC support: no streaming, no custom interceptor
semantics beyond auth/TLS, and the response message is produced the same way
as the JSON `bodyExpression` — a CEL expression that builds a
`google.protobuf.Struct`-compatible value the front end marshals back into the
declared response message type. Bidirectional/streaming ext_authz-style calls,
and any transport requiring per-service custom logic beyond decode → evaluate
→ encode, remain out of scope and should use a dedicated adapter (as authz
does today for Envoy) rather than the generic front end.

**Non-goals for this dialect in the first release:** no gRPC service
*generation* (the binding only serves methods whose request/response types are
already described by the referenced descriptor set); no streaming RPCs; no
generic gRPC client (outbound) support, only inbound serving; no automatic
translation between this dialect and `JSONSchema202012SubsetV1` — a
`PayloadSchema` picks one dialect. Authoring shifts from hand-writing JSON
Schema to pointing at an existing `.proto`/descriptor set, which is a smaller
lift for protobuf-native integrations (Envoy, and any other gRPC-based
external system) and a strictly worse fit for JSON-native ones (nono, generic
webhooks) — hence both dialects, not one.

## Worked Examples and Migrations

### Before / after responsibilities

| Concern | Authz today / nono prototype (`kyverno-nono-design`) | Generic design |
|---|---|---|
| Input model | HTTP native structs; Envoy protobuf; nono's `RequestData` flattened struct | Versioned PayloadSchema plus standard transport decoding |
| CEL types | Native/protobuf registration (`ext.NativeTypes`) | Schema-generated declarations and generic adapters |
| Dispatch | Envoy/HTTP constants; nono's `EvaluationModeNono` constant | One Schema capability plus schema reference |
| Convenience functions | `http.*`, `envoy.*`, nono's `nono.Grant()/Deny()/argv()` and receiver methods | Existing value-based functions and typed `derived` fields |
| Policy result | Authz typed response/null; nono's `*CheckResponse`/null | Boolean validations, generic aggregate decision |
| Wire response | Integration server converts native response | JSON response projection or existing protocol adapter |
| Exceptions/metrics | Integrated independently per server; nono reuses authz's `pkg/events/` wholesale | Shared evaluator and event/metric contract |
| New JSON payload type | New compiler/type integration and release (nono: ~1,100 new Go lines across 4 packages, 1 new binary) | New schema, binding, and policies |

### Envoy CheckRequest: schema and equivalent decision policy

The example models the HTTP authorization **subset** of Envoy's CheckRequest
JSON representation, not authz's native HTTP wrapper. It deliberately permits
unmodeled Envoy envelope properties while exposing only the fields below.
For a production adapter, pin the protobuf-to-JSON convention (including
snake_case versus lowerCamelCase, int64 strings, bytes, and unset field
presence), then publish a complete tested schema. These details cannot be
inferred from a partial example.

```yaml
apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadSchema
metadata:
  name: envoy-http-check-v1
spec:
  dialect: JSONSchema202012SubsetV1
  compatibilityVersion: "1"
  schema:
    type: object
    required: [attributes]
    additionalProperties: true
    properties:
      attributes:
        type: object
        required: [request]
        additionalProperties: true
        properties:
          request:
            type: object
            required: [http]
            additionalProperties: true
            properties:
              http:
                type: object
                required: [method, path]
                additionalProperties: true
                properties:
                  method: {type: string, maxLength: 32}
                  path: {type: string, maxLength: 8192}
                  host: {type: string, maxLength: 512}
                  headers:
                    type: object
                    maxProperties: 200
                    additionalProperties: {type: string, maxLength: 8192}
  match:
    discriminators: []
    requiredPaths:
      - /attributes/request/http/method
      - /attributes/request/http/path
  exposure:
    include:
      - /attributes/request/http/method
      - /attributes/request/http/path
      - /attributes/request/http/host
      - /attributes/request/http/headers
    exclude:
      - /attributes/request/http/headers/authorization
      - /attributes/request/http/headers/cookie
  computed:
    - name: isReadOnly
      expression: object.attributes.request.http.method in ["GET", "HEAD"]
---
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: envoy-public-read
spec:
  evaluation:
    mode: Schema
    schemaRef: {name: envoy-http-check-v1}
    admission: {enabled: false}
    background: {enabled: false}
  failurePolicy: Fail
  validationActions: [Deny]
  validations:
    - expression: >-
        derived.isReadOnly &&
        object.attributes.request.http.path.startsWith("/public/")
      message: Only read-only public requests are allowed
---
apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadBinding
metadata:
  name: envoy-public-read
spec:
  route:
    path: /checks/envoy
    methods: [POST]
  selection:
    schemaRef: {name: envoy-http-check-v1}
  requiredPolicies:
    - name: envoy-public-read
  enforcement:
    requireApplicablePolicy: true
  response:
    format: Decision
```

After projection, the ancestors of the allowlisted HTTP fields can have closed
CEL declarations: runtime validation still permits unknown input fields, but
the projected view does not expose those unknowns. The headers node remains a
typed map.

Representative existing authz validation:

```cel
object.attributes.request.http.method in ["GET", "HEAD"] &&
object.attributes.request.http.path.startsWith("/public/")
  ? envoy.Allowed().Response()
  : envoy.Denied(403).Response()
```

Its decision-only equivalent is the boolean policy above. The simple
`startsWith` rule demonstrates equivalence, **not** a general URL authorization
recommendation: deployments must agree with the upstream application on path
decoding/normalization and test encoded separators and traversal cases.

The generic `/checks/envoy` endpoint accepts JSON and returns Decision JSON. It
is **not** by itself an Envoy ext_authz endpoint. A retained Envoy adapter receives
the gRPC protobuf, normalizes it into this contract, evaluates the schema engine,
and emits CheckResponse: allow → OK, deny → PERMISSION_DENIED/403, internal error
→ the configured fail-closed error response. Envoy's HTTP ext_authz interface
also requires its own status/header response behavior; it is not simply a POST
of a CheckRequest JSON body.

Authz's existing response header/query mutation helpers and protobuf field
presence behavior need explicit parity tests and later adapter capabilities.
No claim is made that replacing type registration removes transport code.
If new integrations already send JSON over the generic HTTP contract, they
require no such adapter.

### Nono approval: verified contract from the working prototype

Unlike the [Envoy example](#envoy-checkrequest-schema-and-equivalent-decision-policy), this is not an illustrative contract. The wire envelope, field
names, zero-valued union, `argv[0]`-is-shim-path convention, and fail-closed
default all come from `pkg/cel/libs/authz/nono/types.go` and
`cmd/kyverno-nono/handlers/approve.go` on branch `kyverno-nono-design`
(`461922d1c`).

The nono POST body (`nonoRequestBody` / `RequestData`,
`pkg/cel/libs/authz/nono/types.go:29-48,68-72`):

```json
{
  "backend": "kyverno",
  "request": {
    "capability_type": "command",
    "request_id": "req-002",
    "session_id": "test-session",
    "child_pid": 1235,
    "command": "kubectl",
    "caller": "claude",
    "args": ["/usr/local/bin/nono-shim", "delete", "namespace", "production"],
    "intercept_rule": "kubectl"
  }
}
```

Endpoint requests use `"capability_type": "endpoint"` and populate `method`,
`path`, `route_id`, `upstream`, `rule_label` instead; `command`/`caller`/
`args`/`intercept_rule` are the fields left zero-valued in that case, and vice
versa for command requests. This flattened, always-present-but-zero-valued
union is exactly why the prototype's struct model needs no null checks — and
exactly what a `PayloadSchema`'s `oneOf` branches ([Missing, Optional, Nullable, and Union Access](#missing-optional-nullable-and-union-access)) must reproduce without
a hand-written Go type per branch:

```yaml
apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadSchema
metadata:
  name: nono-approval-v1
spec:
  dialect: JSONSchema202012SubsetV1
  compatibilityVersion: "1"
  schema:
    type: object
    required: [backend, request]
    additionalProperties: true
    properties:
      backend: {type: string, maxLength: 128}
      request:
        type: object
        required: [capability_type, request_id, session_id]
        additionalProperties: true
        properties:
          capability_type: {type: string, enum: [command, endpoint]}
          request_id: {type: string, minLength: 1, maxLength: 256}
          session_id: {type: string, minLength: 1, maxLength: 256}
          child_pid: {type: integer, minimum: 0}
          command: {type: string, maxLength: 256}
          caller: {type: string, maxLength: 256}
          args:
            type: array
            maxItems: 256
            items: {type: string, maxLength: 4096}
          intercept_rule: {type: string, maxLength: 256}
          method: {type: string, maxLength: 32}
          path: {type: string, maxLength: 8192}
          route_id: {type: string, maxLength: 256}
          upstream: {type: string, maxLength: 512}
          rule_label: {type: string, maxLength: 512}
        oneOf:
          - required: [command]
            properties: {capability_type: {const: command}}
          - required: [method, path]
            properties: {capability_type: {const: endpoint}}
  match:
    requiredPaths: [/backend, /request/capability_type, /request/request_id]
  exposure:
    include: ["/request/**"]
    exclude: []
  computed:
    - name: argv
      # equivalent to the prototype's object.request.argv(): drop args[0],
      # the nono shim path, since generic policies get a plain string function
      # instead of a per-type receiver method (see [Type Provider, Adapter, and Runtime Sketches](#type-provider-adapter-and-runtime-sketches)).
      expression: >-
        size(object.request.args) <= 1 ? [] : object.request.args[1:]
    - name: isMutatingKubectl
      expression: >-
        object.request.capability_type == "command" &&
        object.request.command == "kubectl" &&
        derived.argv.exists(a, a in
          ["apply","delete","scale","patch","edit","create","replace","rollout"])
---
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: nono-kubectl-readonly
spec:
  evaluation:
    mode: Schema
    schemaRef: {name: nono-approval-v1}
    admission: {enabled: false}
    background: {enabled: false}
  failurePolicy: Fail
  validationActions: [Deny]
  validations:
    - expression: >-
        object.request.capability_type != "command" ||
        object.request.command != "kubectl" ||
        !derived.isMutatingKubectl
      message: kubectl is restricted to read-only subcommands
---
apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadBinding
metadata:
  name: nono-approve
spec:
  route:
    path: /approve
    methods: [POST]
  selection:
    schemaRef: {name: nono-approval-v1}
  requiredPolicies:
    - name: nono-kubectl-readonly
  enforcement:
    requireApplicablePolicy: true
    # matches approve.go:66-108: no applicable/matching policy still denies.
    defaultOnNoMatch: Deny
  response:
    format: JSON
    statusCode: 200
    bodyExpression: >-
      decision.allowed
        ? {"decision": "granted"}
        : {"decision": "denied", "reason": decision.message}
```

Before (real prototype code, `pkg/nono/compiler/compiler.go:139-160` and
`pkg/cel/libs/authz/nono/lib.go:41-96`): a hand-written `RequestData` Go
struct, `object.request.argv()` as a compiled member overload backed by Go
reflection (`impl.go:41-52`), and a validation expression that must return
`*nono.CheckResponse` or `null`, built with `nono.Grant().Response()` /
`nono.Deny("...").Response()`. After: the same `argv` drop-shim-path logic is
a `computed` field expressed once in the schema (not Go), validations return
plain booleans per the existing CEL policy compiler's requirement
(`pkg/cel/compiler/compiler.go:136-151`), and the JSON `{"decision":
"granted"|"denied"}` wire shape — which the prototype hard-codes in
`writeGrantedOrDenied` (`approve.go`) — becomes a declarative
`bodyExpression`, reproducible for any future backend that also wants
`{"decision": ...}` semantics without new Go code.

The prototype's fail-closed default (`approve.go:66-108`: no engine result ⇒
denied) is preserved by `enforcement.defaultOnNoMatch: Deny` on the binding
rather than being implicit server behavior — making it an explicit,
reviewable configuration choice instead of something only visible by reading
the Go handler.

This does not remove the need for a real `/approve` HTTP listener, TLS,
PolicyException semantics for break-glass sessions, or the events/metrics
fan-out the prototype currently borrows wholesale from kyverno-authz
(`cmd/kyverno-nono/internal/serve.go:1-45`). Those remain transport- and
observability-layer concerns ([Performance and Observability](#performance-and-observability)), reused across schemas rather than
duplicated per payload type — the generic mechanism replaces the CEL
type/compiler/mode layer specifically, which in the prototype is
`pkg/cel/libs/authz/nono/` (341 lines) and `pkg/nono/compiler/` (280 lines).

The server must still authenticate the agent and bind approvals to the actual
action being executed. An allowlisted argv tuple is not proof of executable
identity: PATH, environment, working-directory integrity, symlinks, retries,
and replay belong to the authenticated agent/protocol contract, exactly as in
the original nono design (`docs/dev/kyverno-nono/DESIGN.md`, §11 notes on
fail-closed and break-glass). If those facts are part of the decision, add
explicit schema fields and policy checks rather than implying that JSON
validation alone provides sandbox enforcement.

### Decision contract, exceptions, and response safety

Internal decision shape:

```json
{
  "allowed": false,
  "reason": "PolicyDenied",
  "message": "This capability is not approved",
  "schema": "nono-approval-v1",
  "schemaDigest": "sha256:...",
  "policyResults": []
}
```

Evaluate all relevant policies (bounded count). A Deny-action validation
failure, or Fail-policy evaluation error with Deny action, denies. Audit/Warn
outcomes remain observational; they cannot override a denial. If no Deny
policy applies, default-deny through the binding's coverage rule.

A full authorized PolicyException explicitly exempts its referenced policy and
counts as an intentional handled outcome for coverage, but never exempts any
other policy. Partial allowed-values exceptions remain scoped data, not
automatic approval. Compile exceptions against the identical schema view and
enforce existing policy-reference/kind rules plus the deployment's exception
namespace controls. Changing an exception invalidates the relevant snapshot.
An exception cannot bypass authentication, selection, payload validation, or
binding readiness.

Policy match conditions remain filters: setting a condition to “safe command”
does not make other commands a policy denial. Use a boolean validation for an
allowlist, as in the example, or ensure the binding coverage rule closes the gap.

JSON response expressions run after aggregation with `decision`, projected
`object`, and `transport`; they cannot change the internal decision. Only
operator-authorized bindings may define them. On pre-validation errors there
is no typed `object`, so use a fixed generic error response, never the normal
body expression. Projection errors or oversize output yield a non-success
fail-closed transport result.

A malicious or incorrect template can still encode a wire-level grant despite
an internal denial. Generic CEL cannot prove the semantics of an unknown
external protocol. Therefore authorization response mappings are trusted
configuration with mandatory deny/error fixture tests; built-in protocol
adapters must enforce a non-overridable denial status. New JSON consumers should
use the canonical Decision format wherever possible.

# Implementation

## Runtime Architecture

### Control plane and request path

```text
 Kubernetes API
   PayloadSchema       ValidatingPolicy       PolicyException       PayloadBinding
          \                   |                     /                    /
           +------------------+--------------------+--------------------+
                                      |
                    watch -> validate -> dependency index
                                      |
                  schema validator + CEL declaration graph
                  selectors + derived/policy/response programs
                                      |
                         immutable serving snapshot
                  schema UID/digest -> policies/exceptions
                  authenticated route -> binding/candidates
                                      |
                                      v
 Incoming supported transport -> authenticate/authorize route
       -> enforce byte/depth/deadline limits -> decode one JSON document
       -> select exactly one allowed schema
       -> validate ORIGINAL JSON against selected schema
       -> read-only projected typed object + trusted transport metadata
       -> exceptions / policy matching / lazy variables / boolean validations
       -> deny-overrides decision aggregation + coverage check
       -> bounded Decision JSON or declarative response projection
       -> transport adapter sends response; async redacted reporting
```

The dedicated generic HTTP server is an optional deployment using shared CEL
packages, not an expansion of the Kubernetes admission webhook's public attack
surface. Add a generic server once; do not add another binary per schema.
Libraries must also be embeddable in authz and usable offline in the CLI.

### Selection algorithm and precedence

There is no implicit “most specific schema wins” rule:

1. **Authorize the route.** Resolve the binding from an exact method/path
   registration and trusted caller identity. Conflicting route registrations
   make the affected bindings unready; registration order never selects one.
2. **Explicit schema wins over discovery by construction.** A binding with
   `schemaRef` has no fallback candidates. It must satisfy that schema's match
   rules and validation, or reject. Do not retry a weaker schema after failure.
3. **Bound shared-route candidates.** Evaluate only names in the binding.
   Index exact transport predicates, discriminator pointer/value pairs, and
   required paths. Omitted predicates mean no restriction, not higher priority.
4. **Apply cheap predicates, then selector CEL.** Across predicate groups use
   AND; values within a single method/path/header value list use OR.
   All `conditions` must be true. A missing discriminator/required path is a
   nonmatch. Selector CEL receives `raw` (dynamic, bounded JSON) and `transport`
   only; it cannot access `object`, `derived`, policy variables, or network I/O.
5. **Require exactly one matching schema.** Zero means reject, two or more mean
   ambiguous/reject. Any selector evaluation error in a surviving candidate
   causes a selection error; do not silently discard it in favor of another
   schema. Give CEL guards and static discriminator filters to authors.
6. **Validate after selection.** Validation failure is not permission to retry
   discovery against a different schema.

Headers are normalized to lowercase names with list-of-string values; matching
requires one exact listed value unless a CEL condition says otherwise.
Path selection uses the server's documented canonical route path. Policy
authors must not confuse `transport.path` with a path contained in the payload.
Forwarded identity headers are not trusted unless the listener authenticates
and authorizes their proxy source.

`requiredPaths` is a lightweight structural fingerprint, not schema validation or
proof of protocol identity. Avoid computing a hash of every property name or
trying every complete schema against each request. Shape-only discovery should
be limited to explicitly opted-in, mutually exclusive candidates. For
authorization, prefer a dedicated route and authenticated principal.

This is intentionally different from policy matching: many policies can apply,
and all are evaluated. Existing policy match-condition false/error precedence
is retained within each policy; it does not establish precedence between
schemas. Do not reuse PolicyException report priority as schema priority.

### Cache, consistency, and lifecycle

Compile off the request path and publish immutable snapshots. Maintain reverse
indices from schema UID/digest to policies, exceptions, and bindings. The
snapshot records both active compiled entries and failed dependencies; a
compilation failure must never silently remove a deny policy.

Suggested cache keys:

* Schema plan: UID + canonical spec digest + compiler compatibility version.
* Policy program: policy UID + relevant-spec digest + schema digest +
  exception-set digest + library/compiler versions.
* Binding plan: binding UID/spec digest + resolved schema/policy identities.

Exclude status-only resourceVersion churn from recompilation. Deduplicate schema
parsing by content only when isolation metadata stays separate. A delete/recreate
with the same name has a new UID and must invalidate old resolutions. Pins bind
to content, while the serving snapshot also records the current UID.

A request acquires one snapshot and uses it through response generation.
On observed schema/required-policy deletion, invalid update, or revoked access,
publish an unready binding before accepting new requests against stale
dependencies. Already-running requests finish against their acquired snapshot
within their deadline. Do not serve an unbounded last-known-good snapshot after
an observed invalid policy change.

Watch delivery is eventually consistent; this mechanism does not provide
linearizable revocation across replicas. Expose observed versions/readiness,
bound disconnected-watch staleness, and document that urgent revocation may
require disabling the ingress route. Start only after all informer caches sync.
For coordinated rollouts, precreate the new schema and policies, then change
the binding in one update.

### Activation roots and include/exclude paths

Policy expressions receive only:

| Root | Meaning |
|---|---|
| `object` | Validated payload, projected and schema-typed |
| `transport` | Typed, authenticated server metadata with allowlisted headers |
| `derived` | Shared schema computed values, lazy and memoized per request |
| `variables` | Existing policy-local lazy variables |
| `exceptions` | Existing allowed-values/image exception context where supported |

No Kubernetes `request`, `oldObject`, or `namespaceObject` is synthesized.
No raw, unfiltered payload root is available to policy/computed expressions.
The raw selector environment is separate and operator controlled.
Alpha requires an object at the payload root; arrays and primitive values are
supported as nested fields. Root-array/scalar contracts can be added later by
declaring the corresponding root CEL type rather than assuming `ObjectType`.

Projection paths use a **JSON Pointer–based pattern language**, not unrestricted
JSONPath:

* `/request/headers` includes that node and its complete subtree.
* `/request/headers/*` selects immediate children and their subtrees.
* `/request/headers/**` explicitly selects the whole subtree, including its root.
* `/commands/*/argv` applies to every array element; no numeric-index projection
  in alpha, so array length/order are preserved.
* `""` or `/**` selects the root. An absent include list means `/**`; an empty
  include list exposes no payload fields.
* JSON Pointer escaping uses `~0` and `~1`. Reserve `~2` for a literal asterisk
  in this pattern dialect; reject unsupported escapes and wildcards embedded
  inside literal segments.
* Exclusion always wins. Including a descendant retains the necessary ancestor
  containers; excluding an ancestor removes the whole branch. Removing a
  primitive array element without removing its array is forbidden.

Compile patterns once into a trie over the reachable schema graph.
An explicit literal path that resolves nowhere is a configuration error;
dictionary wildcards remain runtime filters. After projection, undeclared or
excluded structured fields produce compile errors.

Projection is a **runtime access boundary**, not merely an optimization to CEL
declarations. Dictionary lookup with a computed key, iteration, `size`, equality,
conversion to native values, JSON serialization, and helper arguments must all
observe the projected view. Otherwise `json.encode(object)` could expose fields
that dot-selection hides. Excluded dictionary keys behave as absent.
Never expose the backing unfiltered map through an adapter method.

Projection does not weaken JSON validation: excluded required fields must still
exist and excluded strings must still satisfy their bounds. It also is not
redaction for ingress logs or selector expressions; those require separate
controls. If a computed field needs an excluded secret, reject it rather than
silently allowing privileged computed-field access.

### Eager versus lazy work

Separate three costs that are often conflated:

1. **JSON parsing/validation:** initially decode one bounded object tree and
   validate all of it. This is O(payload size), even with a tiny CEL projection.
2. **CEL environment compilation:** construct only projected reachable
   declarations. Reuse them across policies and requests. Do not declare one
   top-level CEL variable for every JSON leaf.
3. **CEL value adaptation:** expose typed object/dictionary/list views whose
   children adapt on field access. Memoize accessed children for a request.
   Avoid per-request reflection/type registration and a second deep copy.

`interpreter.Activation.ResolveName("object")` resolves a root variable.
An ordinary nested selection is subsequently resolved by the value/provider,
not by repeated `ResolveName("object.a.b")` calls. Therefore use both a small
activation and lazy nested `ref.Val`/field-access implementations.

`types.NewDynamicMap` wraps an existing Go map and uses reflection; it neither
parses JSON lazily nor generates schema declarations. Prefer specialized
string-key map adaptation or schema object views. Start with a correctness
baseline using eager projected maps, then require semantic equivalence from the
lazy implementation. Keep streaming validation/selective parsing as a later
optimization justified by measured payload sizes.

### Computed fields and helper ergonomics

`spec.computed` on the schema has the same ordered, acyclic expression model as
policy variables. Infer each output type at compilation. A field can refer to
`object`, `transport`, and earlier `derived` fields, but not policy `variables`
or exception decisions. Reject duplicate/reserved names, forward references,
cycles, excessive depth, and incompatible output types.

Evaluate each accessed computed field at most once per request, shared across
policies compiled against the same schema. Cache errors as well as values.
Unaffected policies need not fail for an unused computed field. Computation is
deterministic and pure; clocks, randomness, HTTP, Kubernetes lookups, and
side-effecting libraries are unavailable in the schema-mode initial profile.

Use existing CEL string/list/regex/URL facilities first. Add only narrowly
specified **generic** helpers where necessary, such as a path-segment matching
function taking `(string, string)`, or header lookup taking
`(map<string,list<string>>, string)`. Version overloads and account for their
costs. Do not create a dynamic function body per schema.

Examples of intentional source changes:

```cel
// Proposed use of schema data, not native Go methods:
object.command.argv
derived.isApprovedEndpoint
transport.headers["x-request-id"]
```

An `argv()` method that merely returns a JSON array can become a field or
computed alias. An `Argv()` method that parses shell syntax is not equivalent to
splitting on spaces: require the sender's canonical argv array, or a reviewed,
generic parser with explicit semantics. `PathMatches` needs a documented choice
among regex, glob, decoded URL path, and filesystem semantics; do not silently
substitute one for another.

## CEL and Schema Technical Design

### Schema profile and validation

Use a documented **2020-12 subset**, not the Kubernetes structural-schema
dialect by accident. The first profile supports:

* `type`, `properties`, `required`, `additionalProperties`, `$defs`, local `$ref`;
* boolean schemas (`true` / `false`), including forbidden variant fields;
* homogeneous `items`, bounded arrays/objects/strings, numeric limits;
* `enum`, `const`, supported RE2-compatible `pattern`;
* bounded `allOf`, `anyOf`, `oneOf`, and primitive nullable type arrays.

Reject unsupported validation keywords at schema admission, including remote
`$ref`, `$dynamicRef`, recursive references, `unevaluatedProperties`,
`patternProperties`, tuple `prefixItems`, and conditional applicators in alpha.
Document the supported regex subset; accepting a regex with different semantics
is worse than rejecting it. `format` is annotation-only unless a separately
versioned format-assertion profile is selected.

Use a standards-tested validation implementation behind an interface. Spike
whether existing Kubernetes/OpenAPI validation primitives cover the declared
subset exactly; do not advertise 2020-12 compatibility solely because they
validate CRDs. A dependency decision follows conformance tests, not convenience.
No network resolution is allowed during compilation or evaluation.

### Type mapping

| JSON Schema construct | CEL declaration/runtime value | Constraints and caveats |
|---|---|---|
| `string` | `cel.StringType` / CEL string | `format: date-time`, byte encodings, and enums do not silently become timestamp, bytes, or CEL enums |
| `boolean` | `cel.BoolType` | No string-to-bool coercion |
| `integer` | `cel.IntType` / signed int64 | Require representable signed 64-bit values; `1.0` and `1e0` are integral JSON numbers and must be normalized without float64 round-tripping |
| `number` | `cel.DoubleType` / finite double | Precision loss is explicit; reject non-finite/out-of-range conversion, validate bounds against the original number representation |
| `null` | `cel.NullType` / CEL null | Present-null differs from missing |
| Closed object with named properties | `cel.ObjectType` with a generated provider type name | Static field checking; runtime view must implement presence and field retrieval |
| Dictionary with schema-valued `additionalProperties` | `cel.MapType(cel.StringType, T)` | Missing key can error; a dictionary does not statically enumerate arbitrary keys |
| Unconstrained object / `additionalProperties: true` | `map<string,dyn>` | Validate the supported schema, but field-name type checking is intentionally weaker |
| Object with both named fields and arbitrary keys | Initially `map<string,dyn>` for that node | Named-field precision is lost; emit a schema status warning. Prefer a closed envelope with a dedicated dictionary field |
| Array with homogeneous `items` | `cel.ListType(T)` | Preserve order and full list length; large comprehensions require cost/size limits |
| `enum` / `const` | Underlying primitive type | Enforced by schema validation, not a new nominal CEL enum |
| Optional property (not `required`) | Declared field type plus presence support | Absence must not be filled with a Go zero value |
| Nullable field, e.g. `type: [string, "null"]` | `dyn` at the nullable node in alpha | Honest weakening of checking; guards/conversions required before typed operations |
| `oneOf` / `anyOf` | Common type if every branch projects to the same declaration; otherwise `dyn` at the union node | Full validator still enforces exactly one / at least one matching branch; CEL does not provide general discriminated-union narrowing |
| `allOf` | Validator conjunction; merge declarations only when unambiguous | Do not flatten `additionalProperties` or required constraints incorrectly; incompatible field types reject typed compilation |
| Local `$ref` | Resolve/intern a declaration node | Same-document JSON Pointer references only, bounded expansion, reject cycles |

For required-key matching and typed policy compilation, absence, null, and
unknown fields must remain distinct. `additionalProperties` defaults to true
under JSON Schema; do not silently change that default. Warn when it turns an
otherwise strongly typed object into a dynamic map. Authors wanting strict
contracts should explicitly set false at each closed object.

JSON Schema's `additionalProperties` applies within its own schema object; it
does not automatically collect all property declarations from `allOf` branches.
Preserve validation semantics even when generating a merged CEL declaration.

### Numeric representation

Use `encoding/json.Decoder.UseNumber()` or an equivalent lossless decoder.
Reject duplicate object keys, extra trailing JSON documents, invalid encoding,
and excessive nesting before evaluation. Go's default JSON decoder does not
provide all of these guarantees without additional checking.

Normalize numbers according to the selected schema only after exact schema
validation. Do not pass `json.Number` directly to the default CEL adapter and
assume it knows the schema. A union that makes numeric representation ambiguous
must have a deterministic profile rule: choose double for `number`/mixed
numeric unions, int only for an unambiguously integral declaration.

CEL int64 is not arbitrary precision. Large identifiers and money requiring
decimal precision should be modeled as strings with explicit domain rules.
Document floating-point arithmetic behavior for `number` even when the schema
validator can compare exact decimal bounds.

### Type provider, adapter, and runtime sketches

There are two viable approaches:

* **Kubernetes declaration bridge:** reuse `common.SchemaDeclType`,
  `DeclTypeProvider.EnvOptions`, and compatible unstructured-value adapters for
  the supported OpenAPI-like subset. This follows
  `pkg/cel/resource/openapi.go:20-26`.
* **Generic schema provider:** implement `types.Provider` over a compiled
  declaration graph, with `FindStructType`, `FindStructFieldNames`,
  `FindStructFieldType`, and a matching adapter. This handles projection,
  presence, and the explicit JSON Schema profile more directly.

Recommend a schema intermediate representation with a provider backend.
Reuse Kubernetes machinery only where it preserves the chosen semantics
(especially nullable values, maps, field-name escaping, and projection).
The existing VariablesProvider is a useful pattern, not a complete payload
adapter.

Illustrative Go interfaces, not existing APIs or ready-to-compile code:

```go
type SchemaIdentity struct {
    UID    string
    Digest string
}

type CompiledSchema interface {
    Identity() SchemaIdentity
    ValidateJSON(ctx context.Context, value any) error
    EnvironmentOptions() []cel.EnvOption
    NewActivation(value any, transport Transport) (interpreter.Activation, error)
}

type SchemaResolver interface {
    Resolve(ctx context.Context, name, digest string) (CompiledSchema, error)
}

// The concrete snapshot also contains failed policy entries and dependency state.
type SchemaPolicyProvider interface {
    SnapshotFor(schema SchemaIdentity) (PolicySnapshot, error)
}

// A single engine capability, not an integration-specific compile branch.
env, err := base.Extend(
    cel.CustomTypeProvider(schemaProvider),
    cel.CustomTypeAdapter(schemaAdapter),
    cel.Variable("object", cel.ObjectType(schemaTypeName)),
    cel.Variable("transport", transportType),
    cel.Variable("derived", derivedType),
)
```

Type names include a collision-resistant digest and structural path, not
untrusted arbitrary names that can collide with `variables`, CEL built-ins, or
library namespaces. The provider's `types.FieldType` must implement `IsSet` and
`GetFrom` correctly. Declaring a field type does not itself validate or convert
the payload.

The adapter returns lazy, immutable record/map/list views. It must support
`has`, indexing, optional operations, equality, iteration, conversions, and
error behavior consistently with its declared type. It must not grant access
to filtered backing values. Use per-request memoization; do not share mutable
activations among requests or concurrently evaluated policies without explicit
synchronization. Reuse immutable `cel.Program` instances, not request state.

Structured property names that are not usable CEL identifiers need a stable,
documented escaping convention compatible with the chosen provider. Detect
escaping collisions when admitting a schema. Arbitrary dictionary keys use
indexing, e.g. `object.headers["x-auth-token"]`; do not claim that every
JSON property can be written as a CEL dot-selection.

### Missing, optional, nullable, and union access

These are separate issues:

```cel
// Missing property guard; valid when the schema declares optional comment.
!has(object.comment) || object.comment == "approved"

// Present-null needs an additional guard.
has(object.comment) && object.comment != null &&
  object.comment.startsWith("approved")

// Where optional syntax is enabled and its semantics are tested:
object.?comment.orValue("no comment")
```

Optional selection is not a general null-safe navigation operator: a
present-null value is not necessarily an absent optional. Schema validation
enforces a union but CEL type checking does not automatically narrow arbitrary
`oneOf` branches after a discriminator test. Use guarded `dyn` access, separate
schema contracts for truly distinct variants, or computed fields returning one
stable type.

The existing environment's `orValue` compatibility macro can substitute a
fallback for an error (`pkg/cel/compiler/env.go:112-146`). Do not blindly inherit
that behavior for a new fail-closed authorization profile. Define compatibility
version 1 with standard optional semantics and no broad error-swallowing macro;
leave existing modes unchanged. Conformance tests must pin this distinction.

### CEL source references

The repository pins cel-go `v0.31.0`, but upstream source queries for that tag
returned 404 during research. The following contracts were directly checked
against the available **v0.26.0** source, and local Kyverno usage confirms several
of them. Phase 0 must verify exact signatures/behavior against the dependency
actually resolved by this checkout before implementation:

* [`interpreter.Activation` and lazy root bindings](https://github.com/google/cel-go/blob/v0.26.0/interpreter/activation.go#L24-L107):
  `ResolveName`, `Parent`, and lazy `func() any` / `func() ref.Val`.
* [`types.Provider`, adapter alias, and field access](https://github.com/google/cel-go/blob/v0.26.0/common/types/provider.go#L36-L84).
* [`NewDynamicMap` and `NewStringInterfaceMap`](https://github.com/google/cel-go/blob/v0.26.0/common/types/map.go#L35-L77).
* [CEL environment/program API documentation](https://pkg.go.dev/github.com/google/cel-go/cel):
  `Variable`, `ObjectType`, `MapType`, `ListType`, `CustomTypeProvider`,
  `CustomTypeAdapter`, `OptionalTypes`, `ContextEval`, and cost controls.
* [JSON Schema validation specification](https://json-schema.org/draft/2020-12/json-schema-validation)
  and [core/applicator specification](https://json-schema.org/draft/2020-12/json-schema-core):
  numeric types, required properties, composition, references, and
  `additionalProperties`.

The JSON Schema specifications are normative references; their full pages
could not be fetched with the available web-fetch permissions in this session.
Do not treat a web-search summary as a conformance test. In particular,
mathematically integral JSON values such as `1.0` are integers under JSON Schema.

## Failure Modes and Edge Cases

The important boundary is **ingress/contract failure versus policy evaluation
failure**. A schema mismatch is not a boolean validation result and is not a
per-policy `failurePolicy: Ignore` opportunity.

| Condition | Required behavior |
|---|---|
| Malformed JSON, duplicate keys, trailing documents, invalid encoding | Reject before selection; HTTP 400 in generic HTTP service; no policy evaluation |
| Unsupported media type or oversized body | 415 / 413; streaming read limit enforced before allocating full body |
| Unknown route or unauthorized binding | 404 / 401 / 403 as appropriate; never discover a different route |
| No schema matches | Contract rejection (422), not default Kubernetes or untyped JSON evaluation |
| Multiple schemas match | Contract rejection with `AmbiguousSchema` (422); no lexical/order/priority tie-break |
| Selector expression errors or hits cost budget | Fail closed; bounded diagnostic, no fallback schema |
| Missing/not-ready schema, bad digest, required policy missing | Binding NotReady, generic 503; no stale name-only resolution |
| Wrong payload type, required field missing, unknown field with `additionalProperties: false` | 422 schema error; validate even excluded fields |
| Unknown field with `additionalProperties: true` | Accept subject to limits; expose only through allowed projection/dynamic dictionary |
| Unsupported schema keyword, cyclic/remote reference, projection typo | Reject configuration admission; no route becomes Ready |
| Policy compile failure | Surface policy error; required-policy binding unready; additional policy represented as an error entry, never dropped |
| Policy match condition false | Skip that policy; binding coverage may still deny |
| Match condition error, none false | Existing Fail/Ignore semantics: enforce error per actions or skip; apply coverage afterward |
| Validation evaluates false | Enforce `validationActions` regardless of Fail/Ignore |
| CEL evaluation/derived-field error | `Fail`: apply validation actions; `Ignore`: report ignored error, not a successful approval |
| All policies skipped, only audit policies, or all errors ignored | No enforcing coverage; deny by default |
| Full PolicyException | Explicit exemption for referenced policy only; record identity and reason |
| Cost, concurrency, deadline, or memory budget exhausted | Stop and fail closed; capacity errors must not produce an allow response |
| Response projection fails | Error response; no partially written allow response |
| Watch disconnected | Serve only within configured bounded staleness; then unready/503 |
| Schema or policy changed during request | Single acquired snapshot; next request sees the published replacement |
| Report sink unavailable | Decision remains authoritative; bounded queue/drop metrics; optional strict audit mode is a separate deployment decision |

Generic HTTP decision results may use HTTP 200 with `allowed: false` (as a
decision API); ingress/contract/infrastructure failures use non-2xx. Protocol
adapters map these categories to their protocol's fail-closed responses.
Clients must never equate “HTTP request completed” with approval.

The alpha profile permits `failurePolicy: Ignore` to preserve the API's
meaning, but an ignored error does not satisfy coverage. Another applicable,
successful Deny policy may satisfy coverage, while any explicit denial still
wins. Operators requiring strict authorization should enforce `Fail` on every
bound policy. Existing global force-Ignore toggles must not weaken ingress
validation or dependency readiness; schema-mode deployment behavior needs
explicit documentation and tests.

### Security and multi-tenant boundaries

* Limit CRD authorship, binding changes, policy changes, and exception changes
  with RBAC. Schema/match/response definitions are security-sensitive code-like
  configuration even though they are declarative.
* Validate before evaluating; disable network-capable libraries in the initial
  profile. The current vpol environment includes HTTP/resource/global-context
  facilities (`compiler.go:242-304`), so reusing it wholesale is inappropriate.
* No dynamic remote schema fetches, unbounded regexes, arbitrary helper
  registration, or automatic user-chosen policy sets.
* Isolate tenant schemas/bindings and rate limits. A shared discriminator is not
  tenant identity. Reject client-controlled transport identity overrides.
* Redact raw input, headers, computed values, and CEL error details in logs.
  Report bounded reason codes and schema/policy identities instead.

## Performance and Observability

### Hot-path model

```text
T(request) =
  bounded decode + indexed selection + full schema validation
  + adaptation of accessed fields
  + accessed computed expressions
  + selected policy CEL work
  + bounded response encoding
```

With a cached plan, no Go reflection-based type registration, schema compilation,
policy compilation, Kubernetes API lookup, or remote reference resolution
belongs on this path. The initial implementation still allocates the parsed
JSON tree; it does not claim zero-copy parsing.

For a schema with 10,000 properties and a 20-field exposure, declaration work
should follow the projected graph, not flatten every path into a variable.
Schema validation may still traverse all input data. List iteration, equality,
and serialization can intentionally access entire exposed subtrees; laziness
does not make them constant time.

CEL cost must be metered across the request, not reset independently for every
derived field and policy. Combine cel-go runtime cost limits/tracking with a
shared request budget, context cancellation, bounded helper operations, and
payload/schema limits. Do not assume `ContextEval` interrupts an unbounded
custom Go function. Cost-limit APIs and cancellation frequency must be verified
against the resolved cel-go version.

### Starting guardrails and benchmark gates

Proposed defaults, subject to benchmark and threat-model review:

| Limit | Initial target |
|---|---|
| Request body | 1 MiB |
| JSON nesting | 64 |
| Shared-route candidates | 16 |
| Policy count per selected schema | 100 |
| Schema document | 256 KiB, 10,000 compiled nodes |
| Expanded reference graph | 20,000 nodes, no recursion |
| Composition alternatives | 16 per node, bounded total validation work |
| Computed values | 64, dependency depth 16 |
| CEL cost | 100,000 aggregate units/request |
| Request wall time | 100 ms default, server hard ceiling |
| Response | 64 KiB |

Per-schema `maxItems`, `maxProperties`, and string bounds tighten global limits;
they never override server ceilings. Cap total compiled-cache memory and
concurrent requests/compilations. Avoid evicting a currently required schema
without marking its binding unavailable.

Benchmark fixtures must include:

* 1 KiB, 32 KiB, and 1 MiB payloads; depth 4/32/64.
* 100/1,000/10,000-field schemas with tiny and full exposure.
* One/ten/100 policies, shared and independent derived values.
* Large arrays, dynamic dictionary lookups, `oneOf` worst cases, and invalid
  payloads failing near the end.
* Eager map baseline versus lazy projected views, native authz baseline, and
  current untyped JSON mode.
* Steady-state and concurrent schema/policy updates, including first-request
  cache misses and overload.

Report p50/p95/p99 latency, throughput, allocations/op, retained heap, compile
latency, and cost-limit effectiveness on declared hardware. An initial target is
under 10 ms p99 engine time for a 32 KiB payload and ten simple policies, excluding
transport and reporting. This is an acceptance target, not a measured claim.
No release should claim a latency improvement without these measurements.

### Reporting and operational diagnostics

Use common decision events carrying request correlation ID, authenticated
binding identity, schema UID/digest, policy/exception identities, outcome,
bounded reason, and timing. Do not manufacture a Kubernetes object UID for
external requests just to fit resource reports.

Export stage latency, selection ambiguity, invalid-payload counts, compile
failures, cache hits/size, budget exhaustion, binding readiness, watch staleness,
and report drops. Keep payload-derived request IDs, arbitrary hosts/paths, and
schema digests out of unbounded metric labels. Digests belong in structured
events/traces. An `explain`/CLI output should show candidate rejection reasons
without exposing secrets.

Use existing reporting abstractions where they fit; define an external-subject
adapter for `pkg/engine/api` results and event sinks. Do not copy authz
`pkg/events/` into each schema integration or assume Kubernetes resource
background reports can represent arbitrary approvals unchanged.

## Rollout and Implementation Plan

### Phase 0 — prove semantics and close evidence gaps

**Artifacts:** small reviewable prototypes/tests in an implementation branch,
not changes made by this proposal.

* Verify resolved cel-go `v0.31.0` APIs and type/provider behavior.
* Obtain the nono reference design and real wire request/response fixtures.
* Choose the JSON Schema validator with supported-subset conformance tests.
* Prove typed fields, `has`, nullable values, dictionaries, escaping, and
  optional operations against a map-backed object.
* Prove projection cannot be bypassed by dynamic indexing, equality, native
  conversion, serialization, or helper arguments.
* Capture authz behavior: allow/deny/null, multiple policies, no match,
  exceptions, Ignore, headers, protobuf JSON normalization, and transport errors.

**Exit gate:** documented contract differences and golden fixture expectations,
not simply a successful CEL expression over a map.

### Phase 1 — API and shared schema compiler

**Upstream `kyverno/api` (new/changed):**

* `api/policies.kyverno.io/v1alpha1/payload_schema.go` — new resource.
* `api/policies.kyverno.io/v1alpha1/payload_binding.go` — new resource.
* `api/policies.kyverno.io/v1alpha1/common.go` — additive schema reference;
  review the existing alias chain and all served versions.
* `api/policies.kyverno.io/modes.go` — one Schema capability constant.
* Generated API/client artifacts owned by that repository.

**This checkout (new unless noted):**

* `pkg/cel/schema/` — schema IR, dialect validation, reference resolver,
  declaration provider, projected values, numeric conversion, exposure trie.
* `pkg/cel/compiler/` — share ordered expression compilation without changing
  existing mode semantics.
* `pkg/validation/payloadschema/`, `pkg/validation/payloadbinding/` — CRD
  validation and dependency-independent diagnostics. These follow the existing
  `pkg/validation/<resource>/` layout, but CEL vpol validation remains in its
  current package.
* `pkg/webhooks/policy/` — register validation for the new resources.
* `go.mod`/`go.sum` — update the API module; add a validator dependency only
  after selection.

Reuse the principles in `pkg/cel/resource/openapi.go`, not its Kubernetes GVK
discovery path. Do not add the types under local `api/kyverno/v1`, manually edit
`pkg/client/`, or hand-edit generated CRDs.

**Exit gate:** schema/profile fixtures, fuzzed numeric/presence/projection tests,
round-trip/storage tests for embedded schema documents.

### Phase 2 — schema-aware policy compilation and engine

* `pkg/cel/policies/vpol/compiler/compiler.go` — explicit Schema dispatch,
  injected resolver, schema environment builder; reject unsupported modes
  explicitly where appropriate instead of relying on default Kubernetes
  compilation for new consumers.
* `pkg/cel/policies/vpol/compiler/policy.go` — schema activation/evaluation
  without changing the existing Kubernetes/JSON paths.
* `pkg/cel/policies/vpol/validate.go` — mode-aware validation; the present
  unconditional `matchConstraints` requirement must not be copied.
* `pkg/cel/engine/request.go` — separate schema payload request metadata/type;
  do not force it into a fake `unstructured.Unstructured` Kubernetes object.
* `pkg/cel/policies/vpol/engine/{provider,reconciler,engine}.go` —
  schema/policy dependency indices, readiness/error entries, exact schema
  filtering, aggregation, and snapshot consistency.
* `pkg/cel/policies/vpol/autogen/` and its provider callers — explicitly bypass
  Pod autogen for Schema mode. Legacy `pkg/autogen/v1` and `v2` are not the
  extension point.
* `pkg/controllers/policystatus/` — dependency-aware conditions and diagnostics.
* `pkg/engine/api/` — adapt common result/report interfaces only if required;
  do not route schema evaluation through the legacy JMESPath engine.
* `pkg/controllers/webhook/`, background controllers, and
  `pkg/controllers/admissionpolicygenerator/` — exclude Schema policies from
  Kubernetes admission registration, scans, and VAP generation.

**Exit gate:** cross-schema isolation, failed dependency, snapshot/race,
exception, coverage, and existing-mode regression tests. Test an unrelated
Kubernetes policy is never evaluated for a schema request and vice versa.

### Phase 3 — ingress, CLI, and operational integration

* `pkg/payload/` — new transport-neutral route selection and decision service.
* `pkg/controllers/payload/` — new schema/binding control-plane reconciliation
  using the repository's controller-runtime conventions.
* `cmd/payload-server/` — new optional generic HTTP server, shared with
  embedders through `pkg/payload`, not generated per integration.
* `cmd/cli/kubectl-kyverno/` and existing CLI command packages — offline schema,
  binding, and policy evaluation plus candidate/typing diagnostics.
* `pkg/metrics/` and relevant report adapters — shared external-subject events.
* `pkg/toggle/` — gate the new capability during alpha.
* `config/crds/policies.kyverno.io/`, charts/RBAC, and `Makefile` generation
  targets — regenerate from upstream API types and wire installation.
* `test/cli/` and `test/conformance/` — new generic-payload fixtures and
  chainsaw tests for readiness, policy updates, route access, and exceptions.

**Exit gate:** authenticated HTTP integration tests, startup readiness,
backpressure, malformed input/limits, and benchmark targets. Add replay/denial
fixtures for every response mapping.

### Phase 4 — migration packages and transport parity

Publish versioned Envoy and nono example resources plus:

1. A field/helper translation table with semantic differences.
2. Golden request/response fixtures from the actual protocols.
3. A decision comparison tool and shadow evaluation mode.
4. An authz adapter using the generic library without rewriting its gRPC stack.
5. A documented compatibility matrix for response header/query mutations,
   exception results, and no-match behavior.

Start in shadow mode: existing integration remains authoritative while the
schema evaluator reports differences. A difference on error handling or
no-match default is a security-relevant migration item, not benchmark noise.
Cut over a dedicated binding/canary route only after parity is established.
Retain the old mode/binary for rollback; do not rename existing resources in
place or remove existing mode constants.

### Upgrade and downgrade ordering

1. Install compatible controller/server versions and confirm feature support
   across all replicas and webhook consumers.
2. Upgrade CRDs/API integrations; ensure unsupported consumers cannot default a
   Schema policy into Kubernetes mode.
3. Create schemas and policies, wait for readiness, then bind traffic.
4. Use versioned schema names for changes; stage new policy references before
   changing a binding.
5. To downgrade, stop/redirect schema traffic and remove/deactivate bindings
   and Schema-mode policies **before** reverting controllers or CRDs.

Older servers must not silently prune `schemaRef` and proceed. The current
default compiler dispatch makes rolling upgrades particularly important:
feature enablement requires every relevant consumer to recognize or explicitly
reject Schema mode. Enforce deployment/version prerequisites rather than
assuming additive CRD fields alone make execution backward compatible.

### Implementation verification

When implementation begins, run targeted Go unit/race tests for schema/compiler/
engine packages, CLI fixtures, and chainsaw ingress/controller tests. Add
round-trip API tests in the upstream API module and old-mode regression tests
locally. Fuzz schema reference expansion, JSON decoding, dynamic keys, number
conversion, and eager/lazy semantic equivalence.

For implementation PRs, follow repository checks: `make imports fmt`,
`make imports-check fmt-check`, `make codegen-all-code` (or `make codegen-all`
when docs/manifests change), `make verify-codegen`, and the configured
golangci-lint run. Generated CRDs and clients come from generators. Coordinate
user-facing documentation with `kyverno/website`.

This document itself changes no Go/API/generated code and requires no build or
code generation.

## Acceptance Criteria

The design succeeds when an operator can onboard a third, previously unknown
JSON event type with only PayloadSchema, PayloadBinding, and ValidatingPolicy
resources; misspelled exposed fields fail compilation; malformed or ambiguous
input cannot bypass enforcement; unrelated schema policies never execute;
projection is non-bypassable; and measured hot-path costs are bounded.

Authz and nono examples demonstrate the intended data/decision abstraction.
Protocol parity is established by fixtures and adapters, not assumed from a
shared request name or a schema that happens to validate.

## Link to the Implementation PR

N/A

# Migration (OPTIONAL)

Existing Kubernetes and JSON evaluation modes remain unchanged. Authz and nono
integrations should migrate through shadow evaluation and golden protocol
fixtures before traffic is switched to schema mode. Existing binaries and mode
constants remain available for rollback until input typing, decision behavior,
failure handling, response mutation, metrics, and reports reach parity.

The detailed migration packages, canary process, and upgrade/downgrade ordering
are specified in
[Rollout and Implementation Plan](#rollout-and-implementation-plan). No
resource should be renamed in place. Operators create versioned schemas and
bindings, wait for readiness, compare decisions, and cut over a dedicated route.

# Drawbacks

- The design introduces two new cluster-scoped resources and another policy
  compilation/cache lifecycle for operators and maintainers to understand.
- A restricted JSON Schema profile will not accept every valid 2020-12 schema;
  users may need to simplify schemas or isolate dynamic regions.
- Static typing and projection improve correctness but add control-plane work,
  memory use, and invalidation complexity compared with the existing `dyn`
  JSON mode.
- Generic boolean decisions do not initially reproduce every Envoy response
  mutation or integration-specific typed response composition. Transport
  adapters remain necessary for those behaviors.
- Generic response projections are trusted operator configuration. A malformed
  projection can encode a wire-level result incorrectly unless adapters
  enforce denial invariants and fixtures test both allow and deny responses.
- Protobuf descriptor support broadens the implementation and test matrix
  beyond the original JSON-only goal, including presence, `oneof`, maps,
  well-known types, dynamic gRPC dispatch, and descriptor lifecycle.
- Fail-closed ambiguity and readiness rules favor security over availability;
  invalid schema or policy updates may make a binding unavailable.
- The proposal is intentionally substantial. Doing nothing retains simpler
  core APIs but continues the per-integration code, compiler, binary, and
  release pattern demonstrated by kyverno-authz and kyverno-nono.

# Alternatives

| Alternative | Decision |
|---|---|
| Only use current JSON mode with `object: dyn` | Useful baseline, but no first-class contract, static property checks, routing, or lifecycle guarantees |
| One CEL variable per JSON path | Reject: declaration explosion, difficult optional/array semantics, and a poor expression surface |
| Dynamic Go `reflect.StructOf` generation | Reject: runtime reflection complexity and poor fit for unions/dictionaries/projection; a provider is the natural CEL extension |
| Generate protobuf descriptors for every schema | Reject as the *only* mechanism: protobuf presence/numeric/JSON mappings add impedance and do not solve JSON Schema validation for JSON-native payloads. Adopted as an **additional, optional dialect** for protobuf/gRPC-native payloads (Envoy ext_authz) — see [protobuf dialect](#optional-dialect-protobufdescriptorv1-for-grpc-payloads) |
| Generate Go code from schema | Moves manual work but still needs rebuild/release and does not meet configuration-only onboarding |
| `mode: arbitrary-schema-name` | Reject: conflates processor capability and resource identity |
| Full JSON Schema and zero-copy lazy parsing immediately | Defer: excessive semantic and performance risk before correctness baseline |
| Priority/first-match schema selection | Reject initially: silent fallback/overlap can weaken authorization |
| Keep typed CheckResponse return values in generic validations | Reject initially: incompatible with existing boolean Vpol semantics and reintroduces protocol-specific result types |
| Plugin mechanism (Go plugin, WASM, or sidecar processor) | Reject for the initial capability; see full rationale below |

## Plugin-based Alternative: Shape and Rejection Rationale

A plugin mechanism was seriously considered because it is the more general
solution: instead of describing a payload with a schema, an operator would
supply executable code that decodes the payload and produces CEL bindings
(and possibly response encoding) itself. Three concrete shapes were evaluated.

**Shape A — in-process Go plugins (`plugin.Open`).** A `PayloadPlugin` CRD
would reference a `.so` built with `go build -buildmode=plugin`, loaded via
the standard library's `plugin` package, and exposing a known symbol (e.g.
`func Decode(body []byte) (map[string]any, error)` plus optional CEL
`cel.EnvOption`/function registrations). This is the closest analogue to how
`nono.Lib()`/`envoy.Lib()` are wired today (`cel.Lib(&lib{})`), just loaded at
runtime instead of compiled in.

**Shape B — WASM modules.** A `PayloadPlugin` would reference a WASM binary
(similar to Kyverno's existing OCI-artifact patterns for policies/bundles),
executed per request through a WASM runtime (e.g. `wazero`), exporting a
decode/validate function and, optionally, computed-field functions callable
from CEL through a bridge.

**Shape C — external processor (sidecar/service).** A `PayloadPlugin` would
name a gRPC or HTTP endpoint implementing a small "decode and project" service
contract; the Kyverno server would call out to it per request (or per schema,
to prefetch a projection), analogous to Envoy's own `ext_proc`/`ext_authz`
pattern, or to how kyverno-nono is itself an external server called by nono.

**Why all three were rejected for the initial capability:**

1. **It doesn't remove the core problem, it relocates it.** Every one of
   these shapes still requires someone to write and ship code — a `.so`, a
   WASM module, or a sidecar binary — per external payload shape. It replaces
   "new Go package compiled into Kyverno" (the current authz/nono pattern)
   with "new artifact loaded by Kyverno," which is a real improvement in
   *deployment* coupling (no rebuild of the main binary/release) but not in
   *authoring* effort: someone still hand-writes decode/typing logic per
   integration, which is exactly what [The Repeating Integration Pattern](#the-repeating-integration-pattern) and the nono prototype evidence
   ([baseline corrections](#corrections-to-assumptions-in-the-motivating-examples)) show costs ~1,100 lines and a new release cycle today. A JSON/
   protobuf schema removes that authoring step entirely for any payload the
   dialect's keyword profile can express.
2. **Trust boundary and admission-path security.** Kyverno's `ValidatingPolicy`
   evaluation sits directly on the Kubernetes admission path and (for
   authz-style modes) on live authorization decisions. Go's `plugin` package
   executes arbitrary code in-process with the host's full privileges, no
   sandboxing, and no capability restriction; a compromised or buggy plugin
   can crash the process, leak secrets from memory, or bypass every other
   policy. WASM narrows this significantly (memory-isolated, no ambient
   syscalls) but still executes untrusted logic on a security-critical path
   and needs a capability/resource-budget model that does not exist in
   Kyverno today. A schema, by contrast, is data: it can only describe shapes,
   matches, and CEL expressions the existing compiler already validates and
   sandboxes the same way `ValidatingPolicy` expressions are today.
3. **No static verification at compile/admission time.** A `PayloadSchema`
   is compiled once, up front, exactly like a `ValidatingPolicy`: a typo in an
   exposed field name is a compile error, caught before traffic flows,
   pointing at both the schema and the policy expression ([Goals](#goals)). An
   opaque plugin's decode function cannot be statically checked by the CEL
   compiler at all — errors surface only at runtime, on live traffic, which is
   unacceptable for a fail-closed authorization path.
4. **Operational fragility.** Go plugins require the exact same Go toolchain
   version and dependency versions as the host binary (undocumented ABI,
   no unload, poor cross-platform/cross-architecture support, no Windows
   support at all) — this is a well-known, widely documented limitation of
   `plugin.Open` in production Go services, not specific to Kyverno. WASM and
   sidecar processors avoid the ABI problem but add a new runtime dependency,
   a new failure mode (module/sidecar crash or hang independent of the main
   process), and new latency/observability surface on every request.
5. **It reintroduces per-integration result and helper semantics by
   construction.** Nothing stops a plugin from returning a typed
   `CheckResponse`-shaped object or exposing its own bespoke helper functions,
   which is precisely the fragmentation ([Before / After Responsibilities](#before--after-responsibilities), "Convenience functions" row)
   this design exists to remove. A schema-driven `derived`/computed-field
   mechanism ([Computed Fields and Helper Ergonomics](#computed-fields-and-helper-ergonomics)) gives equivalent expressiveness for the actual use cases
   surveyed in the [Envoy](#envoy-checkrequest-schema-and-equivalent-decision-policy) and [nono](#nono-approval-verified-contract-from-the-working-prototype) examples without an escape hatch back to arbitrary code.

**When a plugin-style mechanism would become worth revisiting:** if a
concrete future payload genuinely cannot be described by JSON Schema or a
protobuf descriptor — for example, a binary framing format, a stateful
multi-message protocol, or a transform that requires calling into an existing
non-CEL parser library — a narrowly scoped, sandboxed "custom decoder"
capability (most likely WASM, with an explicit resource budget and a
`PayloadSchema`-shaped output contract it must still satisfy) is the design to
prototype next, not a general-purpose in-process plugin. It should still
project into the same typed `object`/`derived` activation this document
defines, so policies remain plugin-agnostic. This is deliberately deferred:
no known onboarding case from [Worked Examples and Migrations](#worked-examples-and-migrations) requires it yet.

# Prior Art

- **Kyverno JSON mode** already evaluates arbitrary values as `object: dyn`.
  It proves generic JSON evaluation works, but lacks a typed external contract,
  deterministic schema routing, projection, and schema lifecycle.
- **Kubernetes CRD structural schemas and Kyverno OpenAPI-to-CEL declarations**
  demonstrate schema-derived typing and validation inside the current codebase.
- **kyverno-authz HTTP mode** registers native Go structs with CEL tags, while
  its Envoy mode registers generated protobuf messages and serves the real
  Envoy `ext_authz` gRPC API. These are the strongest implementation
  precedents and expose the cost of per-integration type libraries and modes.
- **kyverno-nono** repeats the native-type pattern for nono.sh's HTTP/JSON
  approval protocol, including a dedicated compiler, CEL helper library,
  binary, events, metrics, and fail-closed handler. Its prototype is the direct
  proof case for replacing code-defined payload contracts with schema-defined
  ones.
- **cel-go protobuf support**, `protoreflect`, `protodesc`, and `dynamicpb`
  provide precedent for descriptor-driven protobuf typing without generated Go
  code per integration. Tools such as grpcurl use the same reflection model.
- **WASM and external processing plugins** are established extension patterns
  in proxy and policy systems. They remain possible future escape hatches, but
  are rejected here because known use cases can be represented as schemas and
  CEL.

# Unresolved Questions

1. **Validator and profile size:** which implementation passes the chosen
   conformance subset without excessive dependency footprint? Is explicit
   restricted-profile naming sufficient for operator expectations?
2. **Provider backend:** how much Kubernetes schema/adapter machinery can be
   reused without changing null, property-name, projection, and union semantics?
3. **Versioned API rollout:** should stable Vpol expose the additive reference
   immediately under a gate, or should Schema evaluation initially be admitted
   only through an alpha served policy version? Existing type aliases mean
   version-specific admission alone needs careful review.
4. **Required policy updates:** is binding readiness on every required-policy
   compile error sufficiently available, or is a bounded last-known-good mode
   needed for non-authorization consumers? Never make stale enforcement
   implicit.
5. **Namespaced delegation:** how should tenant identity, schema references,
   policies, exceptions, and bindings be isolated without cross-namespace
   object references becoming an authorization bypass?
6. **Generic response projections:** should alpha allow them at all on
   authorization routes, or require canonical Decision consumers and vetted
   adapters until mapping tests/tooling exist?
7. **Authz compatibility:** what result-merging and response-mutation behavior
   must remain in adapters? Boolean deny-overrides policies are intentionally
   not a drop-in replacement for every typed-response composition.
8. **Nono wire contract:** the [nono example](#nono-approval-verified-contract-from-the-working-prototype) confirms the schema and response encoding
   against the real prototype (`kyverno-nono-design`). Open items are the
   authenticated agent identity, approval lifetime, and replay protections
   that the current prototype does not yet address (it trusts `caller`/
   `session_id` as presented) — these need explicit schema fields and policy
   checks, not implicit trust in JSON contents, before this becomes a
   production integration.
9. **Reporting destination:** external decisions may not correspond to a
   Kubernetes resource. Select a common event/OpenReports integration without
   abusing Kubernetes UID-based aggregation.
10. **Schema reuse versus isolation:** should future versions add policy-set
    bindings for multiple tenants sharing one schema? Alpha intentionally avoids
    label selectors that can silently remove enforcing policies.
11. **Library capability expansion:** if later policies need HTTP/resource
    context, design explicit capability grants, SSRF controls, caching, and
    budgets first. Do not inherit admission-mode privileges by default.

# CRD Changes (OPTIONAL)

This proposal adds two cluster-scoped resources in
`policies.kyverno.io/v1alpha1`:

- `PayloadSchema`, containing the dialect-specific schema or descriptor
  reference, compatibility version, selection hints, CEL exposure paths, and
  computed fields.
- `PayloadBinding`, containing HTTP route or gRPC method selection,
  authentication/authorization constraints, candidate schemas, required
  policies, enforcement defaults, limits, and response projection.

It also adds an optional `schemaRef` to the existing `ValidatingPolicy`
evaluation configuration and defines one reusable `Schema` evaluation mode.
Existing evaluation mode strings and behavior are preserved. References use a
small version-neutral name/digest structure rather than embedding alpha
resource specification types into stable policy APIs.

The complete proposed YAML and API-versioning constraints are specified in
[Proposed API](#proposed-api). API source changes belong in the upstream
`github.com/kyverno/api` repository, followed by dependency and generated CRD
updates in this repository.
