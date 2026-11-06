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

4. Stock CEL has no list `sum()`; `ext.Math()` offers `math.greatest` only, and
   Kyverno's math lib has no list reduction either. Aggregates over payload
   arrays are a real ergonomic gap for `computed` fields.
5. A plain YAML scalar containing `: ` (e.g. a ternary) parses as a mapping;
   expressions in resources need `>-` folding, as the proposal examples use.
6. Compiling policies against the schema at load time works: unknown fields,
   non-bool validations and bad references are rejected before serving.

## Run it
```bash
go run ./pkg/cel/schema/spike -addr 127.0.0.1:8080          # loads pkg/cel/schema/examples
curl -d @pkg/cel/schema/testdata/nono/kubectl-delete-ns.json localhost:8080/approve
curl -d '{"eventType":"deploy.requested","environment":"prod","image":"r/app:latest"}' localhost:8080/events
```
`examples/nono.yaml`: explicit-schema route, two policies, `requiredPolicies`,
declarative `bodyExpression`. `examples/events.yaml`: two schemas on one route
selected by the `/eventType` discriminator (exactly-one-match, no fallback).

## Proposal fields now implemented
PayloadSchema: `dialect` (only `JSONSchema202012SubsetV1`), `compatibilityVersion`,
`exposure.include/exclude` (applied after validation, before CEL), spec digest
(canonical-JSON sha256, key-order independent), and admission-time rejection of
unsupported keywords/remote `$ref` with the offending JSON Pointer.
PayloadBinding: `authorization.allowedPrincipals` (mTLS principal, 401/403/404
with no cross-binding fallback), `limits` (`maxBodyBytes`, `maxDepth`,
`maxCandidates`, `timeoutMillis`), `enforcement.requireApplicablePolicy`.
Readiness rules: every selectable schema needs a required *enforcing* policy;
audit-only policies never block; empty principals or
`requireApplicablePolicy: false` rejected when authorization is on.
`ValidatingPolicy.validationActions` (Deny/Audit/Warn).

Known simplifications: `maxCELCost` is parsed but NOT enforced; `timeoutMillis`
is checked between validations, not inside a running CEL program; no
`status` conditions (digest is only logged); no hot reload; principal comes from
the first URI SAN or the CN of a verified client cert.

mTLS demo: see `examples/` resources plus
`go run ./pkg/cel/schema/spike -tls-cert server.crt -tls-key server.key -client-ca ca.crt`
(client certs need URI SANs `spiffe://example.com/nono-agent` for `/approve`
and `spiffe://example.com/build-system` for `/events`). Without `-client-ca`
the server runs unauthenticated and ignores authorization.

## Not yet covered
- `oneOf` on `request` is ignored by the CEL type bridge (validator only, not tested here).
- Nullable fields, dictionaries, field-name escaping, optional syntax.
- Projection (`exposure.include/exclude`) and bypass resistance.
- Envoy / protobuf.
- Exceptions, response-safety checks, `status` conditions, `maxCELCost` enforcement.
- Real CRDs and the `evaluation.mode: Schema` dispatch inside the vpol compiler (needs kyverno/api).
