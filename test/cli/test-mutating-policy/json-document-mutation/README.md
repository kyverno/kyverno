# JSON mutation in the CLI

These fixtures evaluate JSON-mode MutatingPolicies through the existing
mutation engine, then validate the resulting objects:

```sh
kyverno apply policy.yaml --json input.json
kyverno apply policy.yaml --json input.json,nested/input.json --output-dir patched/
kyverno test .
```

Inputs and outputs must have object roots. No Kubernetes metadata or kind is
required. Nested arrays/mixed values are supported, and integers above 2^53
remain exact within int64. Output uses the normal mutation YAML output path.

Use the declared `jsonPayloads` paths in `results[].resources`.
`patchedResources` compares against each policy's patched object, not the
final output of the last policy.

The neighboring `json-skips-errors` fixture covers match exclusions,
PolicyExceptions, rollback on a failed JSON Patch `test`, and invalid patches.
JSON-mode policy admission in-cluster follows ValidatingPolicy conventions,
including the normal requirement for Kubernetes match constraints.
