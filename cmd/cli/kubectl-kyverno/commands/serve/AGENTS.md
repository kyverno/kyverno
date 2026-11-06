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
- **Never fail open.** Startup uses `evaluator.LoadStrict`, which errors on any load problem.
  `decide` maps no results, or only skips, to `NoMatch` and never to `Allow`. Decode errors are 4xx
  and evaluation errors or panics are 5xx, all with a `Status` body.
- **The response schema in `api.go` is the integration contract** for curl users and the Kyverno
  Playground. Changing a field is a `v1alpha1` API change, so update `docs/user/cli/serve.md` and
  `test/cli/serve/e2e.sh` along with it.
