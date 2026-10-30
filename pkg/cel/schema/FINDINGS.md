# Phase 0 spike findings

Spike for `docs/dev/proposals/schema-driven-cel-processing.md`. Run with
`go test ./pkg/cel/schema/...`. Not wired into any binary.

Approach: JSON Schema -> `spec.Schema` (kube-openapi) -> `common.SchemaDeclType`
-> `DeclTypeProvider` ("Kubernetes declaration bridge" in the proposal), with
kube-openapi's validator for payload validation.

## Works
- Typed `object.request.*` fields; mistyped or undeclared fields fail at compile time.
- `has()` on absent optional fields; reading an absent optional field errors (no zero value).
- Computed fields via a `derived` map, evaluated in order after validation.
- Lossless decode (`UseNumber`), trailing-document rejection, schema validation.

## Contract differences vs. the proposal text
1. `args[1:]` in the proposal's `argv` computed field is not valid CEL. Use
   `ext.Lists()` `args.slice(1, size(args))`.
2. The `argv` expression is not presence-safe: endpoint requests have no `args`,
   so it errors. The prototype's Go struct zero-filled fields, so the claim that
   the schema model "needs no null checks" does not hold; computed fields need
   `has()` guards.
3. kube-openapi's validator rejects `1.0` for `type: integer` when given
   `json.Number` (`TestIntegralFloatPidKnownGap`). Needs a conforming validator or
   pre-validation normalization.

## Not yet covered
- `oneOf` on `request` is ignored by the CEL type bridge (validator only, not tested here).
- Nullable fields, dictionaries, field-name escaping, optional syntax.
- Projection (`exposure.include/exclude`) and bypass resistance.
- Envoy / protobuf.
