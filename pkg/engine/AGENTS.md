# AGENTS.md — pkg/engine

The legacy (`kyverno.io`) rule-evaluation engine was deleted in #17710 PR 4 (handler dispatch, pattern/anchor
matching against admission requests, the `forceMutate` path, and `pkg/webhooks/resource/updaterequest.go`
no longer exist). What remains is a shared library consumed by the CEL engines (`pkg/cel/policies/*`), the
global context controller, image verification, and the CLI — not an engine in its own right.

## What's here

- `api`: the policy-context types (`PolicyContext`, `RuleResponse`, `EngineResponse`) and their copy-on-write
  semantics. `PolicyContext` carries the admission request/old/new resource and is cloned (`Copy()`), not
  mutated in place, when a caller needs an isolated view for a nested evaluation.
- `apicall`: executes `APICall`/context-entry HTTP calls. `executor.go` enforces an egress policy that blocks
  loopback and link-local ranges (including IPv4-mapped/compatible and NAT64-encoded forms of
  `169.254.169.254`, the cloud metadata address) unless explicitly allow-listed — this is SSRF hardening, not
  general robustness, and the blocked-range error intentionally omits the resolved address. `httpclient.go`
  separately handles the scoped ServiceAccount token (custom audience) used for outbound calls.
- `context`: the JMESPath/CEL-context store (`context.Interface`), its `Checkpoint`/`Restore`/`Reset` stack
  used to roll back context mutations between rule evaluations, and the `loaders/` for `apicall`, `configmap`,
  `globalcontext`, `imagedata`, and variable context entries.
- `variables`: `ReplaceAllVars`/`SubstituteAll` and condition evaluation (`AnyAllConditions`, `Condition`,
  `ConditionOperator`). Still imports `kyvernov1` for the condition/rule types kept in the `api/kyverno/v1`
  root after PR 4; it does not evaluate legacy policy rules.
- `jmespath`: the custom JMESPath function registry (`NewFunctionRegistry`) used by `variables` and by CEL's
  `context` library.
- `anchor`, `pattern`, `validate`, `operator`, `wildcards`, `jsonutils`: pattern/anchor matching and JSON
  traversal primitives, originally written for legacy rule validation, now shared by the CLI's JMESPath
  tooling and by nothing else that executes against live admission traffic.
- `mutate/patch`: strategic-merge and JSON-6902 patch generation (`strategicMergePatch.go`,
  `patchJSON6902.go`), used by `pkg/admissionpolicy` to build `MutatingAdmissionPolicy` patches and by the CLI.
- `adapters`: thin adapters from `pkg/clients/dclient`/`pkg/clients/kube` client interfaces to the narrower
  interfaces `apicall`/`context` loaders expect.
- `handlers`: down to `handler.go`'s single helper, `WithResponses(...)`, which filters nil `RuleResponse`s
  into a slice; used by callers that still build engine-style rule responses (CEL `gpol`/`mpol` processors).

## Working here

- This package has no admission-request dispatch loop any more. If you're adding behavior that reacts to an
  `AdmissionReview`, it belongs in `pkg/webhooks` or a CEL engine (`pkg/cel/policies/*`), not here.
- `context.Checkpoint()`/`Restore()`/`Reset()` are paired; a loader or evaluator that checkpoints before a
  speculative mutation must restore or reset on every exit path, including early returns on error.
- `apicall/executor.go`'s egress policy is security-relevant: if you touch host/IP validation there, re-run
  `go test ./pkg/engine/apicall/...` and check the IPv6-encoding test table (`executor_test.go`) covers any new
  bypass you're closing.
