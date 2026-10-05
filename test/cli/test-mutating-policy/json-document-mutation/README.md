# JSON document mutation in the CLI

These fixtures exercise document-native `MutatingPolicy` evaluation with
`spec.evaluation.mode: JSON`, followed by JSON-mode validation:

```sh
kyverno apply policy.yaml --json input.json
kyverno apply policy.yaml --json input.json -o patched.json
kyverno apply policy.yaml --json input.json,nested/input.json -o patched-documents/
kyverno test .
```

Mutation preserves raw JSON, including integer literals above 2^53. Documents
need no Kubernetes metadata or GVK. JSON-mode mutations accept object, array,
scalar, and null roots; the neighboring `json-root-transitions` fixture checks
per-policy outputs across root-type transitions.

`jsonPayloads` entries identify test documents by their exact path as declared
in the test file. Use those paths in `results[].resources`, including directory
components when basenames overlap. `patchedResources` names a complete expected
JSON document (or a single YAML document), compared without converting numbers
through float64. Each mutation result is compared against that policy's output,
not against the last policy's output.

Output follows the existing `apply -o` destination convention. Without `-o`,
documents are printed as a YAML-compatible stream containing JSON values. A
`.json` destination holds exactly one JSON document; multiple documents require
a directory or a `.yaml`/`.yml` stream. Directory output uses
`<input-basename>-<path-hash>-mutated.json`, avoiding collisions between inputs
with identical basenames. Mutation or validation failures do not create or
truncate JSON output files.

Existing JSON validation, image-validation, and deletion engines require object
roots. Combining those policies with a non-object final document produces an
actionable error instead of coercing the document or panicking. MutatingPolicies
without JSON mode are explicitly skipped for JSON input; JSON-mode
MutatingPolicies do not run against Kubernetes resource input.

The neighboring `json-skips-errors` fixture covers match exclusions,
PolicyExceptions, atomic rollback after a failed JSON Patch `test`, and an
explicit error for an invalid patch. Error results must not declare
`patchedResources`, since no successful document exists.
