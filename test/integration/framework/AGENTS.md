# AGENTS.md — test/integration/framework

Shared harness for the envtest-based integration tests in `test/integration/<type>/`. Those test packages carry the
`integration` build tag and need `KUBEBUILDER_ASSETS` (see `.github/workflows/check-framework.yaml` for the envtest
version CI uses). This package itself is untagged, so its own `_test.go` files run under plain `go test`.

## Building an environment

`NewTestEnv(crdPaths...)` starts envtest, a controller-runtime manager, real kube/kyverno/dynamic clients, a dclient
and the real `libs.ContextProvider`. Only the API server is fake; the wiring mirrors the production binaries.
`NewTestEnvWithOptions` builds on it and wires only the policy types passed to `WithPolicyTypes`, exposing each as a
non-nil `env.<Type>` setup. Engines are registered on the manager, so wire them **before** `(*TestEnv).Start`, and
call `Stop` when done. Pass every CRD directory the tests need through `WithCRDPaths` up front: the clients cache
discovery, so installing CRDs mid-test is unreliable. Once `WithCRDPaths` is used, the policies.kyverno.io default
is no longer added for you.

## Background (UpdateRequest) tests for MutatingPolicy

- `NewMpolBackgroundController(t, env, eventGen)` runs the real `background.NewController` (the UpdateRequest
  controller, with its real retry/backoff and 10 workers) over envtest. It needs `WithPolicyTypes(Mpol)` and the
  `config/crds/kyverno` CRDs. It is per test: each call builds a fresh dclient (cold discovery cache, like a just-started
  controller) and stops, then deletes leftover UpdateRequests, on test cleanup. The legacy engine and the gpol
  dependencies are nil, so it only serves `CELMutate` requests.
- The policy controller is **not** run. Tests call `SubmitCELMutateUR`, which creates the same bare request
  `pkg/policy` submits (created, then set `Pending`). `PendingURCount() == 0` means every request completed, because
  the controller deletes completed requests.
- `WithProviderDelay(d)` wraps the mpol provider in `DelayedMpolProvider`, which hides each newly compiled version for
  `d` after its first fetch and keeps serving the previous one meanwhile. This is the only way to reproduce the
  policy-controller-vs-engine lag; the default wiring compiles synchronously. Change it per test with
  `env.Mpol.Delayed.SetDelay` and reset it in `t.Cleanup`. Only `Fetch` is delayed, not the admission-path
  `MatchesMutateExisting`.

## Test isolation

envtest has no namespace controller: a deleted namespace stays `Terminating` and its name can't be reused, and fixed
names collide under `-count`. Use `UniqueName(t, prefix)` / `CreateUniqueNamespace` for anything a race test creates,
and run race tests locally with `-count=20`. `CreateNamespace` keeps its fixed-name behaviour because many existing
tests depend on it. Tests in one package share a single `TestEnv` (and one engine), so they must not run in parallel
against it, and must remove the policies they create.
