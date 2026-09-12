# Integration testing framework for CEL policies

This package (`test/integration/framework`) is the shared harness for writing integration tests
against Kyverno's CEL-based policy types: `ValidatingPolicy`, `MutatingPolicy`, `GeneratingPolicy`,
and `DeletingPolicy` (plus their namespaced variants). It runs each policy through the same
compiler, provider, engine, and admission handler that the production controllers use, backed by a
real Kubernetes API server from [envtest](https://book.kubebuilder.io/reference/envtest.html).

The guiding principle is to use the same code the real controller does and swap only the low-level
Kubernetes API for envtest. Tests create policies and resources through the real API, let the real
reconciler compile them, and call only the public handler methods. That makes a passing test a
faithful statement about production behaviour, not about a mock.

## Contents

- [How it works](#how-it-works)
- [Running the tests](#running-the-tests)
- [Your first test](#your-first-test)
- [Per-policy-type guide](#per-policy-type-guide)
  - [ValidatingPolicy](#validatingpolicy)
  - [MutatingPolicy](#mutatingpolicy)
  - [GeneratingPolicy](#generatingpolicy)
  - [DeletingPolicy](#deletingpolicy)
- [Testing multiple policy types together](#testing-multiple-policy-types-together)
- [Testing reports](#testing-reports)
- [Helper reference](#helper-reference)
- [Conventions](#conventions)
- [envtest gotchas](#envtest-gotchas)

## How it works

envtest starts a real `etcd` and `kube-apiserver`, but no controller-manager, scheduler, or
kubelet. On top of that, the framework wires a controller-runtime manager plus the real Kyverno
clients and context provider:

- `NewTestEnv` / `NewTestEnvWithOptions` start envtest, install the policy CRDs, and build a
  `*TestEnv` that exposes the manager, the typed clients, and (optionally) a per-type setup.
- The per-type setups build the engine and provider through the production constructors
  (`NewKubeProvider` for `ValidatingPolicy`/`MutatingPolicy`, informer-backed listers for
  `GeneratingPolicy`/`DeletingPolicy`), so a policy created through the API is compiled by the same
  reconciler used in the controllers.
- Your test calls the public admission handler (for example `ValidateClustered`) or, for
  `DeletingPolicy`, runs the real controller.

`TestEnv` fields:

| Field | Type | Purpose |
|---|---|---|
| `Env` | `*envtest.Environment` | The running envtest environment. |
| `Mgr` | `ctrl.Manager` | Controller-runtime manager (its client is the shared cache). |
| `Client` | `client.Client` | Cache-backed client for creating policies and resources. |
| `KubeClient` | `kubernetes.Interface` | Typed Kubernetes client. |
| `KyvernoClient` | `kyvernoclient.Interface` | Typed Kyverno client. |
| `DClient` | `dclient.Interface` | Kyverno dynamic client. |
| `ContextProvider` | `libs.Context` | The real CEL context provider (same code path as production). |
| `Vpol` / `Mpol` / `Gpol` / `Dpol` | per-type setup | Non-nil only for the types requested via `WithPolicyTypes`. |

## Running the tests

All integration tests carry the `//go:build integration` build tag, so they are skipped by a plain
`go test ./...` and run only when you pass `-tags=integration`.

They need envtest binaries (`etcd`, `kube-apiserver`) on disk, located through the
`KUBEBUILDER_ASSETS` environment variable. Install `setup-envtest` and resolve the path exactly as
CI does (see `.github/workflows/check-framework.yaml`):

```bash
# Install setup-envtest (once). The version tracks the Kubernetes version below.
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.24.0

# Point KUBEBUILDER_ASSETS at the envtest binaries for the target Kubernetes version.
export KUBEBUILDER_ASSETS=$(setup-envtest use 1.36 -p path)
```

`setup-envtest` is installed into `$(go env GOPATH)/bin`; make sure that is on your `PATH`.

Run a single policy type's suite:

```bash
go test -tags=integration ./test/integration/vpol/...
```

Each package brings its own CRDs, so the command is the same for every type:

```bash
go test -tags=integration ./test/integration/vpol/...
go test -tags=integration ./test/integration/mpol/...
go test -tags=integration ./test/integration/gpol/...
go test -tags=integration ./test/integration/dpol/...
go test -tags=integration ./test/integration/reports/...
go test -tags=integration ./test/integration/multi/...
```

Useful flags:

```bash
# Defeat the test cache and add the race detector.
go test -tags=integration -count=1 -race ./test/integration/vpol/...

# Run one test by name.
go test -tags=integration -run TestValidate_DenyPolicy_BlocksNonCompliantResource ./test/integration/vpol/...
```

The framework package also carries plain unit tests (for the YAML loaders) that need neither envtest
nor the `integration` build tag. They are a fast check that the module builds and the loaders behave,
but they do not validate your envtest setup (`KUBEBUILDER_ASSETS`):

```bash
go test ./test/integration/framework/...
```

## Your first test

A minimal `ValidatingPolicy` test has three parts: a `TestMain` that stands up the environment once,
a policy created through the API, and a public handler call whose response you assert on.

```go
//go:build integration

package vpol_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	vpol "github.com/kyverno/kyverno/pkg/webhooks/resource/vpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testEnv  *framework.TestEnv
	engine   vpolengine.Engine
	provider vpolengine.Provider
)

func TestMain(m *testing.M) {
	var err error
	testEnv, err = framework.NewTestEnv("../../../config/crds/policies.kyverno.io")
	if err != nil {
		panic(err)
	}

	engine, provider, err = framework.NewVpolEngineWithExceptions(testEnv.Mgr)
	if err != nil {
		testEnv.Stop()
		panic(err)
	}

	if err := testEnv.Start(); err != nil {
		testEnv.Stop()
		panic(err)
	}

	code := m.Run()
	testEnv.Stop()
	os.Exit(code)
}

func TestValidate_DenyPolicy_BlocksNonCompliantResource(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-production-pods"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "!has(object.metadata.labels) || object.metadata.labels.?env.orValue('') != 'production'",
				Message:    "production pods are not allowed",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	// Create through the API, then wait for the reconciler to compile it.
	require.NoError(t, testEnv.Client.Create(context.Background(), policy))
	t.Cleanup(func() { _ = testEnv.Client.Delete(context.Background(), policy) })
	require.Eventually(t, func() bool {
		policies, err := provider.Fetch(context.Background())
		return err == nil && len(policies) >= 1
	}, 5*time.Second, 100*time.Millisecond, "policy not reconciled")

	// Drive the real admission handler.
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("prod-app", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "prod-app", "namespace": "default", "labels": {"env": "production"}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.False(t, resp.Allowed, "production pod should be denied")
}
```

The existing `handler_test.go` in each package already defines small helpers for the create-and-wait
step (`createPolicyWithCleanup`, `waitForPolicyReady`, `waitForPolicyGone`). Reuse them rather than
re-implementing the polling in every test.

## Per-policy-type guide

Each policy type has a matching test package under `test/integration/`. `ValidatingPolicy`,
`MutatingPolicy`, and `GeneratingPolicy` are exercised by calling their admission handler directly.
`DeletingPolicy` is schedule-driven, so its test runs the real controller instead.

### ValidatingPolicy

Kyverno's `ValidatingPolicy` validates Kubernetes resources or JSON payloads and is a superset of the
Kubernetes `ValidatingAdmissionPolicy`.

Wiring:

```go
engine, provider, err := framework.NewVpolEngine(testEnv.Mgr)                // no exceptions
engine, provider, err := framework.NewVpolEngineWithExceptions(testEnv.Mgr)  // PolicyException support
```

Drive it through the handler and assert on the response:

- `h.ValidateClustered(ctx, logr, request, "", time.Now())` for a `ValidatingPolicy`.
- `h.ValidateNamespaced(...)` for a `NamespacedValidatingPolicy`.
- `resp.Allowed` is the admit/deny decision; `resp.Warnings` carries `Warn`-action messages;
  `resp.Result.Message` holds the failing validation's message.

`ValidationAction` selects the outcome: `Deny` blocks, `Audit` admits but records a result, `Warn`
admits but returns a warning.

### MutatingPolicy

`MutatingPolicy` mutates new or existing resources.

Wiring (note it needs the context provider and kube client, and the `reports` CRDs):

```go
engine, provider, err := framework.NewMpolEngine(ctx, testEnv.Mgr, testEnv.KubeClient, testEnv.ContextProvider)
engine, provider, err := framework.NewMpolEngineWithExceptions(ctx, testEnv.Mgr, testEnv.KubeClient, testEnv.KyvernoClient, testEnv.ContextProvider)
```

The mutate response carries a JSON patch. Decode it and assert on specific operations rather than on
"something changed":

```go
patches := decodePatches(t, resp.Patch)          // []jsonpatch.JsonPatchOperation
op := findPatch(patches, "/metadata/labels/env") // first op at that path, or nil
require.NotNil(t, op)
assert.Equal(t, "add", op.Operation)
assert.Equal(t, "production", op.Value)
```

`decodePatches` and `findPatch` live in `mpol/handler_test.go`. `MutatingPolicy` tests load two CRD
directories (`config/crds/policies.kyverno.io` and `config/crds/reports`) and install a pass-through
reports breaker in `TestMain`, because the handler's report path calls
`breaker.GetReportsBreaker()`.

### GeneratingPolicy

`GeneratingPolicy` creates or clones resources based on flexible triggers. It does not mutate the
triggering object at admission; instead the handler emits an `UpdateRequest`, and a background
processor creates the downstream resources.

Wiring uses informer-backed listers:

```go
gpolLister, ngpolLister := framework.NewGpolListers(ctx, testEnv.KyvernoClient)
engine, provider := framework.NewGpolEngine(gpolLister, ngpolLister)
```

There are two ways to observe the result:

- `framework.MockURGenerator` captures the `UpdateRequest` specs without acting on them. Use it when
  you only want to assert that the correct request was produced.

  ```go
  urGen := &framework.MockURGenerator{}
  h := gpol.New(urGen, gpolLister, ngpolLister, "")
  resp := h.Generate(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp(name, ns, admissionv1.Create, raw), "", time.Now())
  // ... poll urGen.GetSpecs() and assert on the captured spec.
  ```

- `framework.ProcessingURGenerator` runs each captured request through a processor that actually
  creates the downstream resources in envtest, so you can assert on the generated object.

  ```go
  processor := framework.NewURProcessor(engine, provider, testEnv.ContextProvider)
  urGen := framework.NewProcessingURGenerator(processor)
  ```

For clone-and-sync scenarios, build the real watch manager with `framework.NewGpolWatchManager` and
use `framework.NewURProcessorWithSyncWatchers`, which applies the same `DeleteDownstreams` and
`SyncWatchers` wiring as the background controller.

### DeletingPolicy

`DeletingPolicy` deletes matching resources based on a schedule. It is not an admission handler; it
runs inside the cleanup controller. The test therefore builds the controller's dependency graph and
runs it.

```go
// Start the environment first: the deps need the running dclient and REST mapper.
testEnv.Start()
deps := framework.NewDpolDeps(ctx, testEnv.DClient, testEnv.KyvernoClient, testEnv.KubeClient, testEnv.Mgr.GetRESTMapper(), testEnv.ContextProvider)
go deps.Controller.Run(ctx, deleting.Workers)
```

Create target resources and a `DeletingPolicy` with a short `Schedule` (for example `* * * * *`),
trigger execution, then poll the API for `apierrors.IsNotFound`. Assert on events through
`deps.EventCapture.GetEvents()`; call `deps.EventCapture.Clear()` at the start of each test to
isolate them. Use `NewDpolDepsWithExceptions` when a test needs `PolicyException` support; it also
returns the `PolicyException` informer so you can prime its cache before triggering.

## Testing multiple policy types together

`NewTestEnvWithOptions` wires only the policy types you request into a single envtest binary, which
is the way to test interactions across types (for example a mutation followed by a validation).

```go
testEnv, err := framework.NewTestEnvWithOptions(
	framework.WithPolicyTypes(framework.Vpol, framework.Mpol, framework.Gpol),
	framework.WithExceptionsEnabled(),
)
```

After `Start`, the requested types are exposed as non-nil setups and unrequested types stay nil:

```go
testEnv.Vpol.Engine, testEnv.Vpol.Provider
testEnv.Mpol.Engine, testEnv.Mpol.Provider
testEnv.Gpol.Engine, testEnv.Gpol.Provider, testEnv.Gpol.Lister, testEnv.Gpol.NamespacedLister
testEnv.Dpol.Deps // nil here, since Dpol was not requested
```

Options:

| Option | Effect |
|---|---|
| `WithPolicyTypes(...)` | Selects the types to wire. Additive; duplicates are ignored. |
| `WithExceptionsEnabled()` | Enables `PolicyException` support for every wired type. |
| `WithCRDPaths(...)` | Overrides the CRD directories. Defaults to `config/crds/policies.kyverno.io`. |

## Testing reports

Admission report emission is a process-wide singleton that `NewTestEnv` locks to "disabled" unless
you turn it on first. Call `framework.EnableReporting()` **before** `NewTestEnv`, and load the
`reports` and `policyreport` CRDs:

```go
func TestMain(m *testing.M) {
	framework.EnableReporting() // must come before NewTestEnv

	testEnv, _ = framework.NewTestEnv(
		"../../../config/crds/policies.kyverno.io",
		"../../../config/crds/reports",      // EphemeralReport (admission emits these)
		"../../../config/crds/policyreport", // PolicyReport (aggregation persists these)
	)
	// ...
}
```

`framework.AggregateEphemeralReports` reproduces the reports controller's single-pass, stateless
aggregation for one resource, without running any controller or queue. It lists the ephemeral
reports for a resource UID, runs the real production merge, and returns a `PolicyReport` you can
persist and assert on. Populate the matching field of `aggregate.Maps` with the active policy key
(`cache.MetaObjectToName(policy).String()`), the same key the controller uses.

## Helper reference

All helpers live in `framework/helpers.go` unless noted.

Admission requests:

| Helper | Builds |
|---|---|
| `PodAdmissionRequest(name, ns, raw)` | Pod `CREATE`. |
| `PodAdmissionRequestDryRun(name, ns, raw)` | Pod `CREATE` with `DryRun` set. |
| `PodAdmissionRequestWithUsername(name, ns, user, raw)` | Pod `CREATE` with a custom `UserInfo.Username`. |
| `PodAdmissionRequestWithOp(name, ns, op, raw)` | Pod with a given operation (`DELETE` puts the object in `OldObject`). |
| `NamespaceAdmissionRequest(name, raw)` | Namespace `CREATE` (a cluster-scoped trigger). |

Capturing side effects (all snapshots are thread-safe; use the getters, not the fields, because the
handlers spawn async goroutines):

| Helper | Captures |
|---|---|
| `MockEventGen{}` → `GetEvents()` | Events emitted by a handler. |
| `MockURGenerator{}` → `GetSpecs()` | `UpdateRequest` specs, unprocessed. |
| `NewProcessingURGenerator(processor)` → `GetSpecs()`, `ProcessingErrors()` | `UpdateRequest` specs, after the processor creates downstream resources. |

Other helpers:

| Helper | Purpose |
|---|---|
| `PodMatchRules()` / `PodMatchRulesWithOps(ops...)` | `MatchConstraints` for pods. |
| `CreateNamespace(t, kubeClient, name)` | Creates a namespace (with the `kubernetes.io/metadata.name` label) and registers cleanup. |
| `ContextWithPolicies(ctx, names...)` | Injects policy names the way the webhook server routes them to handlers. |
| `LoadGeneratingPolicy(t, path)` (`loaders.go`) | Decodes a single-document YAML file into a `GeneratingPolicy`. |
| `LoadResource(t, path, obj)` (`loaders.go`) | Decodes a single-document YAML file into any typed object. |

## Conventions

- **Build tag.** Test files that drive envtest start with `//go:build integration`, so a plain
  `go test ./...` skips them. The framework's own `loaders_test.go` is a pure unit test and is
  intentionally left untagged.
- **One environment per package.** Stand the environment up once in `TestMain` and call `Start`
  exactly once; it panics on a second call. Tests share it, so give each policy a unique name.
- **Name tests by user scenario.** `TestValidate_DenyPolicy_BlocksNonCompliantResource` reads better
  than `TestValidateClustered_ReturnsFalse`; the name should say what would break for a user.
- **Assert on specifics.** Decode the patch and check the value, check the event reason, check the
  report summary. Avoid `assert.NotNil(resp)`, which proves nothing.
- **Clean up with `t.Cleanup`, mind the order.** `t.Cleanup` runs last-in-first-out. For
  `PolicyException` tests, delete the exception first, let the re-reconciliation settle, then delete
  the policy, in a single cleanup function, otherwise the exception delete re-queues the policy while
  you are deleting it.

## envtest gotchas

envtest runs only `etcd` and `kube-apiserver`. Several behaviours differ from a real cluster and can
fail a test silently:

- **No garbage collection.** A `Foreground` delete leaves the object in `Terminating` forever
  because no controller finalizes it. Use `metav1.DeletePropagationBackground` for any delete you
  assert on.
- **Namespace labels.** Real clusters inject `kubernetes.io/metadata.name` on every namespace;
  envtest does not. `CreateNamespace` sets it for you, so `NamespaceSelector` matching by name works.
  If you build a namespace by hand, set the label yourself.
- **Cluster-scoped blast radius.** A cluster-scoped `DeletingPolicy` or `GeneratingPolicy` with an
  empty match lists every namespace in the cluster, which affects parallel tests. Pin a
  `NamespaceSelector` (or a `matchExpressions In: [test-ns]`) to the test's own namespace.
- **Async event emission.** The validate and mutate handlers record events in a goroutine that runs
  after the call returns. Wait briefly (for example `time.Sleep(200 * time.Millisecond)`) before
  asserting on `GetEvents()`.
- **Default ServiceAccounts and Endpoints are not created.** Create them explicitly if a test needs
  them.
- **CEL constant folding.** `1/0` is folded at compile time and will not produce a runtime error. To
  force a runtime evaluation error, access a missing field on a dynamic type, for example
  `object.neverExists.attr`.

For the full list, see `test/integration/framework` alongside the per-package `handler_test.go`
files, which document the specific gotcha each test works around.
