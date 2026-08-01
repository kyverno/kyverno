# Admission webhook guidance

`pkg/webhooks` receives Kubernetes `AdmissionReview` requests and returns
admission decisions. It is on the synchronous request path between the
Kubernetes API server and Kyverno, so correctness, latency, and failure mode
are all part of a change's behavior.

## Entry points

- `server.go` constructs the TLS HTTP server and registers policy, resource,
  exception, CEL exception, and global-context routes.
- `handlers/` provides reusable request-processing middleware such as
  filtering, protection, metrics, tracing, and admission handling.
- `resource/` contains handlers for resource admission, including policy types
  and image verification. `policy/`, `exception/`, and `globalcontext/` handle
  their respective Kyverno resources.
- `utils/` builds policy context and contains shared matching, reporting, and
  error helpers.

## Testing

- Add or update focused tests beside the changed handler and run
  `go test ./pkg/webhooks/...` while iterating.
- Run `make test-unit` for changes that affect shared webhook behavior.
- Validate externally visible admission behavior with an appropriate
  `test/conformance/chainsaw` scenario when possible; changes to routes,
  webhook configuration, or enforcement behavior need cluster-level coverage.

## Change safety

- Keep handler composition consistent: filtering, managed-resource protection,
  authorization information, metrics, and tracing are deliberately applied at
  the server boundary.
- Do not change admission outcomes, route paths, or error handling without a
  reproducible policy/resource proof in the PR.
- Be careful when modifying code shared by validating and mutating paths, or by
  cluster-scoped and namespaced policy types.

## Automation boundary

Automation may label webhook changes and recommend focused tests, but must
escalate route, enforcement, authentication/authorization, and managed-resource
protection changes for human review.
