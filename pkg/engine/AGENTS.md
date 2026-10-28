# AGENTS.md — pkg/engine

The legacy (`kyverno.io`) rule-evaluation engine was deleted in #17710 PR 4 (handler dispatch, pattern/anchor
matching against admission requests, and the `forceMutate` path no longer exist). `pkg/webhooks/resource/
updaterequest.go` was removed earlier, by #17710 PR 1 (#18164). What remains is a shared library consumed by
the CEL engines (`pkg/cel/policies/*`), the global context controller, image verification, and the CLI — not
an engine in its own right.

## What's here

- `api`: shared result/policy types — `EngineResponse`, `PolicyResponse`, `RuleResponse`, `GenericPolicy`
  (wraps both admission-policy and CEL policy kinds behind one interface), `GenericException`, and the
  `Client`/`ResourceClient`/`RegistryClient` interfaces `adapters` implements. There is no
  `PolicyContext` type or `Copy()`/copy-on-write mechanism in this tree any more.
- `apicall`: executes `APICall`/context-entry HTTP calls. `executor.go` enforces an egress policy that blocks
  loopback and link-local ranges (including IPv4-mapped/compatible and NAT64-encoded forms of
  `169.254.169.254`, the cloud metadata address) unless explicitly allow-listed — this is SSRF hardening, not
  general robustness, and the blocked-range error intentionally omits the resolved address. `httpclient.go`
  separately handles the scoped ServiceAccount token (custom audience) used for outbound calls.
- `context`: down to `context/resolvers`, two `engineapi.ConfigmapResolver` implementations
  (informer-based and client-based) used to fetch ConfigMaps for context-entry resolution. The JMESPath/CEL
  context store, its `Checkpoint`/`Restore`/`Reset` stack, and the `apicall`/`configmap`/`globalcontext`/
  `imagedata`/variable loaders that used to live here were deleted with the legacy rule-evaluation engine.
- `variables` no longer exists: `ReplaceAllVars`/`SubstituteAll`, condition evaluation
  (`AnyAllConditions`/`Condition`/`ConditionOperator`), and `variables/regex` were deleted with the legacy
  rule-evaluation engine and `api/kyverno/v1` (#17713 section 3) and no longer exist here.
- `jmespath` no longer lives here: the custom JMESPath function registry moved to `pkg/jmespath`
  (it serves GlobalContextEntry projections, not rule evaluation, so the `pkg/engine` prefix was
  misleading).
- `anchor`, `pattern`, `validate`, `operator`, `wildcards`: pattern/anchor matching and JSON
  traversal primitives, originally written for legacy rule validation. Still live in the admission path:
  `anchor` is imported by `wildcards`, `validate`, and `mutate/patch` (which is itself on the
  live `ivpol`/`mpol` admission handlers — see below); `pattern` is imported by `validate`, and
  `operator` by `pattern`.
- `mutate/patch`: strategic-merge and JSON-6902 patch generation (`strategicMergePatch.go`,
  `patchJSON6902.go`). Real importers are `pkg/webhooks/resource/ivpol/handler.go` and
  `pkg/webhooks/resource/mpol/handler.go` — it is on the live admission request path, not a CLI-only helper.
- `adapters`: thin adapters from `pkg/clients/dclient` and `github.com/kyverno/sdk/extensions/registryclient`
  to the narrower `Client`/`ResourceClient`/`RegistryClient` interfaces in `api`. Imported by
  `pkg/admissionpolicy/{mutate,validate}.go` and `pkg/controllers/globalcontext/controller.go`.
- `handlers`: down to `handler.go`'s single helper, `WithResponses(...)`, which filters nil `RuleResponse`s
  into a slice; its real callers are `pkg/cel/policies/vpol/engine` and `pkg/cel/policies/mpol/engine`.

## Working here

- This package has no admission-request dispatch loop any more. If you're adding behavior that reacts to an
  `AdmissionReview`, it belongs in `pkg/webhooks` or a CEL engine (`pkg/cel/policies/*`), not here.
- `context.Checkpoint()`/`Restore()`/`Reset()` are paired; a loader or evaluator that checkpoints before a
  speculative mutation must restore or reset on every exit path, including early returns on error.
- `apicall/executor.go`'s egress policy is security-relevant: if you touch host/IP validation there, re-run
  `go test ./pkg/engine/apicall/...` and check the IPv6-encoding test table (`executor_test.go`) covers any new
  bypass you're closing.
