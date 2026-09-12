# Kubernetes client access

Two, differently-purposed generated layers exist. Picking the wrong one is an easy mistake for anyone (human or
agent) new to the repo, and nothing currently stops it at compile time — this doc is the fix for that.

## `pkg/client/` — raw generated typed access to Kyverno's own CRDs

Generated clientset, listers, and informers — produced by `client-gen`/`lister-gen`/`informer-gen` via
`make codegen-client-all`. It covers not just the CRDs defined in this repo's `api/` (`kyverno.io`, `policyreport`,
`reports.kyverno.io`) but also the externally-defined `policies.kyverno.io/v1beta1` types from
`github.com/kyverno/api` (see `Makefile`'s `codegen-client-clientset`/`-listers`/`-informers` targets, which take
`github.com/kyverno/api/api/policies.kyverno.io/v1beta1` as an input alongside this repo's own `api/` packages).
Every file carries a `// Code generated ... DO NOT EDIT.` header. **Do not import this package from business
logic.** It exists only as the base that `pkg/clients/` wraps.

## `pkg/clients/` — the instrumented wrappers business logic actually uses

A second generation layer, produced by `hack/client-wrapper` via `make codegen-client-wrappers`: metrics + tracing
+ logging middleware wrapped around both `k8s.io/client-go` and `pkg/client/clientset`. These files are generated
too (`*.generated.go`, `interface.generated.go`) even though — unlike everything else generated in this repo — they
currently carry no `DO NOT EDIT` header. Treat them as no-edit anyway; a `.claude/settings.json` with
`permissions.deny` enforcing this by path is planned (not added yet — see
[docs/designs/agent-friendly-restructure.md](../../designs/agent-friendly-restructure.md)).

Within `pkg/clients/`, one thing is hand-written and is the preferred entry point for new code:

- **`pkg/clients/dclient`** (`dclient.Interface`) — a unified, hand-written dynamic + discovery client for
  arbitrary GVKs. This is what almost all new controller/webhook/engine code should use for generic resource
  access, and is imported pervasively across `cmd/*/main.go` and `pkg/controllers/*`. It has no metrics/tracing
  decorators of its own — but every controller binary (`kyverno`, `cleanup-controller`, `reports-controller`,
  `background-controller`) constructs it through `cmd/internal.Setup` (`createKyvernoDynamicClient` in
  `cmd/internal/client.go`), which hands it dynamic and kube clients that are *already* wrapped with
  `WithMetrics`/`WithTracing` first (`cmd/internal/setup.go`). So ordinary resource calls made through `dclient` in
  those binaries do carry that instrumentation by delegation; it just isn't decorated at the `dclient` layer itself,
  which matters for any `dclient` constructed outside `cmd/internal.Setup` — the `kubectl-kyverno` CLI's `apply`
  command and `ext/cluster.New` both do this directly, with no instrumentation at all. See
  [pkg/clients/AGENTS.md](../../../pkg/clients/AGENTS.md) for more.
- **`pkg/clients/{kube,kyverno,dynamic,metadata,apiserver,aggregator}`** — generated, instrumented typed wrappers.
  Use these when the exact Kubernetes type is known and a typed client reads more clearly than `dclient`.

## Rule of thumb for new code

1. Need to CRUD/watch/list an arbitrary or dynamically-discovered GVK? → `pkg/clients/dclient.Interface`.
2. Need a specific, statically-known Kubernetes or Kyverno type? → the matching typed wrapper under `pkg/clients/`.
3. Never *construct* a client from `pkg/client/clientset` or raw `k8s.io/client-go` directly — that bypasses the
   metrics/tracing/logging instrumentation every other client call site gets. Accepting one of those packages'
   *interface* types as a function parameter is fine (the generated wrappers embed those same interfaces); the rule
   is about what constructs the concrete client, not what type a signature names.
