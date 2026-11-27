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
   `json.Number`. Fixed in the spike by rewriting integral values for integer-typed
   nodes before validation (`numbers.go`); numbers then become int64 or float64 by
   the *declared* type, not by the value. A conforming validator is still open.

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

Known simplifications: `limits.maxCELCost` is rejected at load (the repository's
per-program cost limit and per-request budget always apply; no per-binding
override); no `status` conditions (digest is only logged); no hot reload;
principal comes from the first URI SAN or the CN of a verified client cert.

## Repository rules the spike now follows
- Every CEL program is built with `compiler.NewProgram` and run with `ContextEval`
  on a context from `requestbudget.Ensure` (`TestNoUnbudgetedCELEvaluationPaths`).
  `timeoutMillis` is a real context deadline, so it interrupts a running expression.
- Envs come from `compiler.NewEnv`. Response bodies use `DynamicResourceEnvOptions`
  because `{"allowed": true, "schema": decision.schema}` is a mixed-type literal.
- Logging is structured (`pkg/logging`), not the standard `log` package.

## Review-driven fixes
- nono kubectl is an allowlist, not a denylist. The first version blocked a list of
  mutating verbs, so `cordon`, `exec`, `drain`, `label` and `cp` were all granted. Now
  only `get`, `describe`, `logs`, `top`, `explain`, `api-resources`, `api-versions`,
  `version` and `cluster-info` pass, `args[0]` must be the shim (`endsWith('nono-shim')`)
  and the verb must come directly after it. Global flags before the verb are refused on
  purpose (`-n get delete pod x` fools a "first word" check). This over-denies, never
  under-denies. Both the shim name and the allowlist are assumptions to confirm against
  the real nono. A full path to kubectl (`/usr/bin/kubectl`) is covered too. Endpoint
  requests require `method` and `path`, command requests `command` (schema `oneOf`).
- Duplicate JSON keys and over-deep nesting are rejected before schema selection.
- Exposure: an explicitly empty `include` exposes nothing (nil means everything);
  `*` and `**` path wildcards work.
- Routes are keyed by method and path; an unknown method is a 404, not a 405.
- Failures before a typed object exists (bad JSON, no schema, validation failure)
  get a fixed denial body and never run the binding's `bodyExpression`. A binding
  whose template gives the same output for allowed and denied is rejected at load.
- `response.statusCode` is validated; `-client-ca` without `-tls-cert` fails fast.
- A bare `derived.x` (typed `dyn`) is accepted as a boolean validation.
- Tuple-form `items` (an array of schemas) is rejected: the profile check did not look
  inside it, so an unsupported keyword there was silently dropped.
- `NewServer` sets `ReadTimeout`, `WriteTimeout` and `IdleTimeout`. Before, only the
  header read had a deadline, so a client could send one byte of body and stall for ever
  (`TestStalledBodyIsCutOff`).
- Audit policies never change a decision, whether they pass, fail or error
  (`TestAuditPolicyNeverChangesTheDecision`).

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
