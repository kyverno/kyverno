# Kubernetes client access

Two, differently-purposed generated layers exist. Picking the wrong one is an easy mistake for anyone (human or
agent) new to the repo, and nothing currently stops it at compile time — this doc is the fix for that.

## `pkg/client/` — raw generated typed access to Kyverno's own CRDs

Generated clientset, listers, and informers for the CRDs defined in this repo's `api/` (`kyverno.io`,
`policyreport`, `reports.kyverno.io`) — produced by `client-gen`/`lister-gen`/`informer-gen` via
`make codegen-client-all`. Every file carries a `// Code generated ... DO NOT EDIT.` header. **Do not import this
package from business logic.** It exists only as the base that `pkg/clients/` wraps.

## `pkg/clients/` — the instrumented wrappers business logic actually uses

A second generation layer, produced by `hack/client-wrapper` via `make codegen-client-wrappers`: metrics + tracing
+ logging middleware wrapped around both `k8s.io/client-go` and `pkg/client/clientset`. These files are generated
too (`*.generated.go`, `interface.generated.go`) even though — unlike everything else generated in this repo — they
currently carry no `DO NOT EDIT` header. Treat them as no-edit anyway; `.claude/settings.json`'s `permissions.deny`
enforces this by path regardless of the missing header.

Within `pkg/clients/`, one thing is hand-written and is the preferred entry point for new code:

- **`pkg/clients/dclient`** (`dclient.Interface`) — a unified, hand-written dynamic + discovery client for
  arbitrary GVKs. This is what almost all new controller/webhook/engine code should use for generic resource
  access, and is imported pervasively across `cmd/*/main.go` and `pkg/controllers/*`.
- **`pkg/clients/{kube,kyverno,dynamic,metadata,apiserver,aggregator}`** — generated, instrumented typed wrappers.
  Use these when the exact Kubernetes type is known and a typed client reads more clearly than `dclient`.

## Rule of thumb for new code

1. Need to CRUD/watch/list an arbitrary or dynamically-discovered GVK? → `pkg/clients/dclient.Interface`.
2. Need a specific, statically-known Kubernetes or Kyverno type? → the matching typed wrapper under `pkg/clients/`.
3. Never: `pkg/client/clientset` directly, or raw `k8s.io/client-go` — both bypass the metrics/tracing/logging
   instrumentation every other client call site gets.
