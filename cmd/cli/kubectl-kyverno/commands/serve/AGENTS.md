# AGENTS.md — cmd/cli/kubectl-kyverno/commands/serve

`kyverno serve` is an experimental local HTTP server. It exposes the CLI evaluation path through
the `v1alpha1` API. It is registered only when `KYVERNO_EXPERIMENTAL=true` is set
(`commands/command.go`). The user-facing contract is in `docs/user/cli/serve.md`.

- **Evaluation runs only through `evaluator.Evaluate`.** Do not add policy evaluation here. Put it
  in `../../evaluator` so `kyverno apply` gets it too. `TestApplyParity` runs the `apply` command
  and `serve` against `test/cli/serve/` and compares their summaries.
- **Requests are serialized (`Server.mu`).** `Evaluate` depends on process-global state. That
  includes `data.InjectProcessor` and the CEL library context that the processors set. Each request
  builds a new `store.Store`, `variables.Variables`, and in-memory client in `Server.Evaluate`.
  Never share them across requests. `TestConcurrentRequestsDoNotLeakState` must pass under `-race`.
- **`POST /v1alpha1/evaluate` (EvaluationRequest)** maps each field to the matching `apply` flag:
  `context` is written to a temp file and passed as `ContextPath` (apply's `--context-file`), which
  is the only thing `resource.get` reads offline. `clusterResources` only seed the fake client.
  `TestEvaluateRequest` checks each field against `apply` run with the equivalent flag.
- **Process-global state reset per request.** Request CRDs land in the global CRD processor, so
  `evaluateWith` calls `resetCRDs` before and after each evaluation. `TestRequestCRDsDoNotLeak`
  checks this.
- **Consumers.** The Kyverno Playground backend (`kyverno/playground`, `backend/pkg/serve` and
  `backend/pkg/playground`) talks to this API over HTTP. Its `scripts/e2e-kyverno-serve.sh` checks
  that `apply`, serve and the Playground agree on `test/cli/serve` fixtures.
- **Never fail open.** Startup uses `evaluator.LoadStrict`, which errors on any load problem.
  `decide` maps no results, or only skips, to `NoMatch` and never to `Allow`. Decode errors are 4xx
  and evaluation errors or panics are 5xx, all with a `Status` body.
- **The response schema in `api.go` is the integration contract** for curl users and the Kyverno
  Playground. Changing a field is a `v1alpha1` API change, so update `docs/user/cli/serve.md` and
  `test/cli/serve/e2e.sh` along with it.
