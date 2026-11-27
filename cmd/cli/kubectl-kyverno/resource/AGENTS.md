# CLI resource loading

## Null preservation

`YamlToUnstructured` and `ExpandIfList` use `normalizeResourceNilMaps` in
`resource.go`. Preserve both the presence and the null value of custom fields:
an absent field, an explicit `null`, and `{}` can produce different policy results.

- Recognized, non-list built-in GVKs retain recursive `normalizeNilMaps`
  compatibility handling for empty map fields emitted by some Helm charts.
- For other resources, normalize only top-level `metadata`. Do not infer the
  schema of custom fields from names such as `data`, `labels`, or `annotations`.
- Do not recursively normalize a list wrapper before expanding its items;
  generic lists can contain custom resources whose nulls must survive.

## List expansion

Keep list recognition in `isListResource`; a `List` suffix alone does not prove
that an object is a list. In `ExpandIfList`, a recognized typed list supplies
missing item kinds and API versions before normalization. Preserve explicit
item GVKs. Generic `List` items do not inherit a GVK from the wrapper.

Normalize each item using its resolved GVK, then recursively expand nested
lists. This preserves custom nulls while retaining built-in map normalization,
including `ConfigMapList` items that omit their GVK.

## Regression coverage

Run `go test ./cmd/cli/kubectl-kyverno/resource -count=1` from the repository root.
`null_fields_test.go` covers custom fields, nested lists, metadata, and built-in
list compatibility. Assert field presence as well as nil values when testing
explicit null preservation. The CLI fixture at
`test/cli/test-validating-policy/preserve-custom-null-fields` and its Chainsaw
test distinguish a nullable custom field from an empty object during evaluation.
