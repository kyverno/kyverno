# JSON mutation

**Status:** implementation proposal; not released.

This proposal is maintained in this repository while the KDP process is being
replaced.

## Scope

Extend `MutatingPolicy` JSON evaluation to match the existing `ValidatingPolicy`
model. Inputs and outputs are JSON objects represented by
`unstructured.Unstructured`; they do not need Kubernetes metadata, a kind, or
an API version. Nested arrays and mixed JSON values are supported, but
array/scalar/null roots and root-type transitions are out of scope.

```yaml
apiVersion: policies.kyverno.io/v1
kind: MutatingPolicy
metadata:
  name: default-region
spec:
  evaluation:
    mode: JSON
  mutations:
  - patchType: JSONPatch
    jsonPatch:
      expression: >
        [JSONPatch{op: "add", path: "/region", value: "us-west-2"}]
```

```shell
kyverno apply policy.yaml --json input.json
kyverno apply policy.yaml --json input.json --output-dir patched/
kyverno test test/cli/test-mutating-policy/json-document-mutation
```

## Existing paths, not a second engine

- The existing mutation compiler dispatches between JSON and Kubernetes
  compilation based on `spec.evaluation.mode`, as the validation compiler does.
- The existing provider compiles policies once and can be reused across inputs.
- The existing mutation engine handles `EngineRequest.JsonPayload` directly,
  without constructing admission attributes for CLI JSON input.
- Results use the existing `EngineResponse` and `MutatingPolicyResponse`.
  Per-policy patched resources retain independent snapshots.
- CLI apply/test use the existing payload loader, processor, mutation output,
  result comparison, and exception reporting. JSON mutations run before JSON
  validation; image-validation and deletion evaluate the resulting object.
- Output follows normal mutation output conventions (YAML on stdout or in
  output files). Directory output uses `<policy-name>-mutated.yaml` for JSON
  payloads without resource names. Multiple inputs append documents as in the
  existing mutation output path. There is no batch-wide output transaction.

## In-cluster behavior

JSON-mode MutatingPolicies follow the same lifecycle as JSON-mode
ValidatingPolicies. JSON mode is not itself a reason to reject policy storage,
remove the policy from providers, disable webhooks, bypass normal matching, or
add special controller cleanup logic.

Policy admission retains the existing requirements, including `matchConstraints`
with resource rules. The CLI's JSON path does not require these constraints.
When a policy is evaluated through admission, normal resource matching still
applies and the compiler evaluates its JSON expressions against the object.
Kubernetes-only mutation features unsupported by the JSON compiler produce
validation/compilation errors rather than being silently reinterpreted.

This change does not add a different cluster-storage contract for mutation or
change the existing validation-policy lifecycle.

## Expression and patch contract

`object` is the current JSON object. Variables are lazy and are refreshed after
each mutation. Match conditions run once per policy. Policies run in provider
order; each sees the preceding policy's successful output.

Only `JSONPatch` mutations are supported in JSON mode. Expressions return a
list of `JSONPatch{op, path, from, value}` values and can use
`jsonpatch.escapeKey()`. The RFC 6902 operations add/remove/replace/copy/move/test
and RFC 6901 escaped pointers are supported. A failed `test` rolls back that
policy and reports a skip; invalid operations report errors. Non-object
mutation outputs are errors.

Integer literals within int64 remain exact through loading, evaluation and
output, including values above 2^53. Fractional JSON numbers use the existing
double representation. Patch `test` comparisons normalize CEL doubles through
their shortest round-trip decimal representation and compare numeric values
exactly, so `0.1` matches `0.1` without conflating adjacent large integers.

Full PolicyExceptions use existing exception result reporting. Scoped
exceptions expose their allowed values/images to expressions. Audit
annotations are attached to successful rule results.

CEL programs use the shared Kyverno environment, cost limits, request-wide
budget and cancellation checks. Documents/intermediate values are limited to
1 MiB and a policy to 1,024 patch operations.

## Tests and follow-up

Focused engine tests cover compilation through the normal provider, sequential
mutations, independent snapshots, numeric fidelity, failed-test rollback, root
restrictions, cancellation and budget/size limits. CLI fixtures cover mutation
before validation, path-based result selection, exceptions, skips and errors.
Existing Kubernetes mutation tests exercise the unchanged evaluation branch.

Arbitrary-root JSON transformations, transactional batch publication and
additional expression libraries can be proposed separately.
