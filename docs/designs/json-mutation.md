# JSON document mutation

**Status:** in implementation; document-native core and CLI apply/test integration
and policy lifecycle isolation implemented on the `feat/json-document-mutation`
worktree branch. This feature is not released.

This proposal is maintained in this repository at the maintainers' request while
the KDP process is being replaced. It is not a claim that the feature is released.

**Related work:** [kyverno/kyverno#13751](https://github.com/kyverno/kyverno/issues/13751),
[kyverno/kyverno#13913](https://github.com/kyverno/kyverno/pull/13913) (closed, unmerged),
and the existing [non-Kubernetes design](https://github.com/kyverno/KDP/blob/main/proposals/kyverno_non_k8s.md).

## Motivation and scope

Kyverno supports JSON Patch mutation of Kubernetes resources, but not mutation
of arbitrary JSON documents through the CLI. ValidatingPolicy already supports
`spec.evaluation.mode: JSON`. Extend the CEL MutatingPolicy model to support
stateless JSON transformations, without representing documents as Kubernetes
AdmissionReviews or requiring resource discovery, GVKs, namespaces, or OpenAPI.

The existing external `github.com/kyverno/api` dependency defines the JSON mode
field for MutatingPolicy. No new policy kind or patch type is necessary.
The document evaluator is separate from the Kubernetes compiler and engine.
Kubernetes routing must explicitly exclude JSON-mode policies.

## Policy syntax

The document compiler accepts this policy without Kubernetes match constraints:

```yaml
apiVersion: policies.kyverno.io/v1
kind: MutatingPolicy
metadata:
  name: default-region
spec:
  evaluation:
    mode: JSON
  matchConditions:
    - name: region-missing
      expression: "!has(object.region)"
  mutations:
    - patchType: JSONPatch
      jsonPatch:
        expression: >
          [JSONPatch{op: "add", path: "/region", value: "us-west-2"}]
```

This example is accepted by the core API and CLI JSON mutation path:

```shell
kyverno apply policy.yaml --json input.json -o output/
kyverno test test/cli/test-mutating-policy/json-document-mutation
```

The output contains the transformed JSON document. CLI result summaries remain
separate from document output. Multiple inputs retain stable file identities;
same-basename files must not overwrite each other.

For one document, `-o patched.json` writes JSON. Directory output uses
`<basename>-<path-hash>-mutated.json`, where the hash is the first four SHA-256
bytes of the input path, formatted as eight lowercase hexadecimal characters.
Multiple inputs may also be written to a YAML output file.

Test result resource identifiers are the exact declared `jsonPayloads` paths.
`patchedResources` compares the complete document produced by that policy:

```yaml
jsonPayloads:
  - nested/input.json
results:
  - policy: default-region
    isMutatingPolicy: true
    resources:
      - nested/input.json
    patchedResources: nested/expected.json
    result: pass
```

JSON-mode MutatingPolicies are excluded from Kubernetes input; non-JSON
MutatingPolicies skip JSON input with an explicit result. Mutation errors can be
tested with `result: error`. YAML payloads and expected documents must contain
exactly one YAML document.

### Field and expression contract

- `object` is the current JSON value. It may be an object, array, string, number,
  boolean, or null. A mutation can change the root type.
- Variables are lazy, declared in dependency order, and refreshed after each
  mutation so references to `object` see the latest value.
- Match conditions run once per policy. A false condition produces a skipped
  result. Evaluation errors are explicit, regardless of admission failurePolicy.
- JSONPatch expressions use the existing `JSONPatch{op, path, from, value}` CEL
  type and `jsonpatch.escapeKey()`. Mixed-type JSON maps/lists are permitted.
- PolicyException references are matched by policy kind/name, as in the existing
  provider. Full exemptions skip mutation; partial allowedImages/allowedValues
  are exposed through `exceptions`. Exception results include copied exceptions.
- Audit annotations are evaluated on the final document; empty strings and null
  are omitted.
- Only JSONPatch is accepted. ApplyConfiguration, enabled Pod-controller/native
  admission-policy autogen, mutate-existing enabled, target matching, server-side
  apply, and non-Never reinvocation are rejected. Empty/defaulted autogen objects
  are accepted.
- Kubernetes match constraints, admission/background switches, webhook settings,
  and failurePolicy do not control this stateless core. Admission defaults must
  not cause a JSON policy to enter the Kubernetes evaluation plane.
- The initial environment includes shared pure CEL extensions, but deliberately
  does not expose `request`, `oldObject`, `namespace`, typed `Object` constructors,
  or network/cluster-context libraries. Broader JSON-mode library compatibility
  can be added explicitly, not through implicit client dependencies.

## Core API and architecture

`pkg/cel/policies/mpol/compiler.CompileJSON` returns an immutable `JSONPolicy`.
It uses shared CEL options, variable-provider support, and matching helpers, but
does not use Kubernetes patch.Request, runtime.Object, or type conversion.

`pkg/cel/policies/mpol/engine.NewJSONEngine(policies, exceptions)` compiles a fixed
policy set. `HandleJSON(ctx, json.RawMessage)` returns a `JSONResponse` containing
the transformed document and per-policy outcomes (`applied`, `skipped`, `error`).
Policy names are namespace-qualified where appropriate. Duplicate keys and
non-JSON policies are constructor errors. Empty policy sets are identity
transformations of valid, size-bounded JSON input.

```go
eng, err := mpolengine.NewJSONEngine(policies, exceptions)
if err != nil {
    return err
}
response, err := eng.HandleJSON(ctx, json.RawMessage(input))
if err != nil {
    return err
}
// response.Document is the transformed JSON, including literal null if appropriate.
```

Policies run in caller-supplied order, exactly once. This avoids silently imposing
alphabetical order on an existing CLI workflow. A CLI using discovered policies
must establish deterministic ordering before constructing the engine.

For expected-output comparisons, each `JSONPolicyResponse.Document` contains the
raw document after that policy ran, including unchanged output for skipped
policies. The CLI uses one ordered engine invocation and consumes these snapshots
only after overall success, without evaluating a policy twice. Snapshots from
earlier successful policies may be present in the engine's error response for
diagnostics; they are not a successful invocation output.

Each call owns its activation, variables, input copy, and patch state. Compiled
programs are shared safely between concurrent calls.

## Patch semantics, atomicity, and limits

All six JSON Patch operations are supported. Paths use RFC 6901 pointers:
empty is the document root; invalid escapes and negative array indices are
rejected. Array indices must be canonical (no leading zeroes or plus signs).
Required constructor fields are checked, including an explicit `from`
for copy/move even when it is the empty root pointer.

The existing direct dependency `github.com/evanphx/json-patch/v5` applies patches.
A private `{"document": <input>}` envelope and translated pointers make scalars,
arrays, and null addressable without depending on the library's root support.
The envelope is never exposed to CEL or callers.

Mutations and policies see previous successful mutations. A failed `test`
operation rolls back the **entire current policy** and returns skipped, retaining
Kyverno's no-op convention while avoiding partial mutation. Removing the root
is rejected because an absent document is not a JSON value. Moving a value into
its descendant is also rejected.

Test comparisons follow JSON structural equality, including numeric equality
(`1`, `1.0`, and `1e0` compare equal). Testing a missing location fails, including
a missing key tested against null; absent and null are not conflated.

Any other evaluation, patch, annotation, cancellation, or limit error stops the
invocation. The top-level response carries outcomes, but **no final document**.
Per-policy snapshots from earlier successful policies remain available for
diagnostics; the erroring policy's snapshot is absent. Consumers must check the
invocation error before publishing any document. The caller's input is never
modified. Literal JSON null is distinct from an absent error result.

Initial core limits:

- One JSON value per invocation; input and each intermediate document are at most
  1 MiB.
- At most 1,024 patch operations per policy, across all its mutations.
- Each CEL program uses Kubernetes' runtime CEL cost ceiling and context
  interruption checks. This is a per-expression budget, not a request-wide SLA.
- Patch-value conversion has a size/node budget; patch serialization and output
  growth are checked. Copy operations have a per-call size-growth limit.

Consumers must still bound policy count and supply context deadlines. Network
operations are unavailable. These limits are initial core constants, not new
cluster feature flags.

## Numeric fidelity

Input/output uses raw JSON bytes. The CEL activation decodes integers as int64,
not float64, so integers above 2^53 remain exact within the signed int64 range.
Out-of-range integer literals and overflowing doubles return errors instead of
silently rounding. Fractional/exponent-form values use CEL doubles; arithmetic
therefore follows IEEE 754 semantics. Untouched numeric lexemes retain their
precision through patching; patch values serialize native CEL integers directly,
without protobuf Value's float64 conversion.

Duplicate object keys currently follow the JSON decoder's last-key-wins behavior.
Whitespace and property ordering are not preserved as a public contract.

## Delivery phases and acceptance

1. **Document-native core (this change).** Dedicated compiler/engine, root-value
   support, sequential evaluation, exceptions, atomic failures, limits, and unit
   tests. No existing admission, provider, or CLI call site is redirected.
2. **CLI apply/test integration (implemented).** Route JSON-mode MutatingPolicies
   through the new engine; preserve raw documents of every root type; reuse
   output paths and expected patched-document fixtures. Run mutation before JSON
   validation and report mode mismatches explicitly. Document identities derive
   from input paths, not Kubernetes metadata. Stage output until evaluation
   succeeds, avoiding truncation or partially published mutation results.
   JSON validation/image-validation/deletion paths that require object roots
   return actionable errors when given a non-object transformed document.
3. **Policy lifecycle isolation (implemented).** JSON policy validation uses the
   document compiler without requiring resource match constraints or admission
   evaluation. A shared mode gate excludes JSON policies from the Kubernetes
   compiler/provider, webhook resource rules, Pod autogen, native admission-policy
   generation, reporting resource scans, and background mutation. Reconciliation
   evicts policies switched to JSON mode from the Kubernetes provider. Status
   reconciliation treats webhook configuration and cluster mutation RBAC as
   inapplicable. Already queued background requests fail explicitly rather than
   performing document policies against cluster resources.
4. **Documentation and release.** Update the website policy comparison and JSON
   examples, generate affected schemas/manifests if necessary, and publish
   end-to-end CLI examples. SDK/HTTP service integration is a later project.

Core acceptance includes every root type, root-type transitions, all patch
operations, escaping, null values, precision above 2^53, policy ordering, variable
refresh, matching/exceptions, test rollback, missing paths, cancellation, size
and operation limits, and concurrent evaluation under the race detector.
Existing mpol compiler/engine tests must continue to pass.

## Alternatives and non-goals

Fake admission attributes were explored in the unmerged implementation, but
couple documents to Kubernetes serialization and mutate-existing semantics.
A shared EngineRequest refactor would unnecessarily change all other policy
engines. Both are avoided in the core implementation.

Generic merge/ApplyConfiguration semantics require a separate design; no
schema-free SSA substitute is introduced. This work does not add HTTP endpoints,
background mutation, Kubernetes reports for documents, legacy policy support,
or convergence with the separate kyverno-json product.
