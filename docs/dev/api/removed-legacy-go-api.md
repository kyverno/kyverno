# Removed legacy Go API (1.20)

This page lists the Go packages and symbols that Kyverno 1.20 removes from this repository as part of
[issue #17713](https://github.com/kyverno/kyverno/issues/17713), and the replacement each consumer should use.

**The Kubernetes APIs stay served.** This removal only affects Go import paths, generated clients, and the
in-tree engine code that used them. The legacy `kyverno.io` CRDs (`ClusterPolicy`, `Policy`, `CleanupPolicy`,
`ClusterCleanupPolicy`, `PolicyException`) keep working through the 1.20 compatibility period: you can still
create, read, update, and delete these objects through `kubectl` or any Kubernetes client, and
`config/crds/kyverno/` stays byte-identical to the pre-1.20 schema (see
[`config/crds/kyverno/README.md`](../../../config/crds/kyverno/README.md)). Writes to these legacy kinds are
denied at admission time as part of the separate 1.20 deprecation policy; see
[`pkg/deprecations/AGENTS.md`](../../../pkg/deprecations/AGENTS.md) for that behavior.

## Removed import paths

| Removed path | What it held |
|---|---|
| `github.com/kyverno/kyverno/api/kyverno/v1` | `ClusterPolicy`, `Policy`, and the legacy rule schema (`Rule`, `ContextEntry`, conditions, `ResourceFilters`, and so on) |
| `github.com/kyverno/kyverno/api/kyverno/v1beta1` | `UpdateRequest` v1beta1 |
| `github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1` | Generated typed client for `v1` |
| `github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1` | Generated lister for `v1` |
| `github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1` | Generated informer for `v1` |
| `github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1` | Instrumented client wrapper for `v1` |
| `pkg/client`/`pkg/clients` typed clients, listers, and informers for `v2` `CleanupPolicy`, `ClusterCleanupPolicy`, `PolicyException` | Generated access for the doomed `v2` roots |
| `pkg/client`/`pkg/clients` typed clients, listers, and informers for `v2beta1` `ClusterPolicy`, `Policy`, `CleanupPolicy`, `ClusterCleanupPolicy`, `PolicyException` | Generated access for the doomed `v2beta1` roots |

`v2`, `v2alpha1`, and `v2beta1` still exist: they now hold only `GlobalContextEntry` (all three versions) and
`UpdateRequest` (`v2`).

## Removed symbols and their replacements

| Removed symbol | Replacement |
|---|---|
| `kyvernov1.APICall` | `kyverno.APICall` (`github.com/kyverno/kyverno/api/kyverno`) |
| `kyvernov1.ServiceCall`, `Method`, `RequestData`, `HTTPHeader` | `kyverno.ServiceCall`, `kyverno.Method`, `kyverno.RequestData`, `kyverno.HTTPHeader` |
| `kyvernov1.ResourceSpec` | `kyverno.ResourceSpec` |
| `kyvernov1.Any` | `kyverno.Any` — same JSON marshaling behavior; see the field-level note below |
| `kyvernov1.AdmissionOperation`, `Create`/`Update`/`Delete`/`Connect` | `admissionv1.Operation` and its constants (`k8s.io/api/admission/v1`) |
| `kyvernov1.ClusterPolicy`, `Policy`, `kyvernov2.CleanupPolicy`, `ClusterCleanupPolicy` | No Go replacement. If you still need the served legacy object, read it as `unstructured.Unstructured` through `dclient` |
| `kyvernov2.PolicyException` | `policiesv1beta1.PolicyException` in the external `github.com/kyverno/api` module, for the CEL exception path. There is no Go replacement for the legacy `v2beta1.PolicyException` kind |
| `kyvernov1.ContextEntry`, `Rule`, `Condition`, `AnyAllConditions`, `ConditionOperator(s)`, `ConfigMapReference`, `GlobalContextEntryReference`, `ImageRegistryCredentials` | None. These types backed the deleted legacy rule-evaluation engine (`pkg/engine/variables`, `pkg/engine/context/loaders`, `pkg/engine/apicall/apiCall.go`) and have no reachable use |

If you were switching a field from `kyvernov1.Any` to `kyverno.Any`, add
`+kubebuilder:pruning:PreserveUnknownFields` alone at the use site. Don't also add
`+kubebuilder:validation:Schemaless`: the root `kyverno.Any` type doesn't carry that marker on its own `Value`
field the way `kyvernov1.Any` did, and adding both markers strips `type: object` from the generated schema.

## What didn't move

Nothing was moved into `api/kyverno` to keep a legacy consumer compiling. Every `pkg/` consumer of a removed
v1/v1beta1 type was either deleted outright (because it had no reachable caller) or rewritten against a
standard Kubernetes type (`admissionv1.Operation`) or the root `api/kyverno` package. See
[`api/AGENTS.md`](../../../api/AGENTS.md) for the rule that keeps `api/kyverno` the only home for types shared
across versions.
