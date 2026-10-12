# AGENTS.md — pkg/controllers/admissionpolicygenerator

Generates native admission policies (ValidatingAdmissionPolicy / MutatingAdmissionPolicy and their bindings) from
Kyverno policies, using the builders in `pkg/admissionpolicy` (see its AGENTS.md for the build-time invariants). This
file covers the controller side for `NamespacedValidatingPolicy` (nvpol), which differs from every cluster-scoped kind.

## Queue keys

- Cluster-scoped kinds use `<Kind>/<name>` strings, which `controllerutils.Run` splits with
  `cache.SplitMetaNamespaceKey`.
- nvpol uses `cache.ExplicitKey("NamespacedValidatingPolicy/<namespace>/<name>")`. A plain three-part string would be
  rejected by `SplitMetaNamespaceKey` and the item dropped after retries. `reconcile` parses it with
  `parseNamespacedPolicyKey`. Always enqueue nvpols through `enqueueVP` or the same explicit key.

## Ownership and cleanup

The generated VAP and binding are cluster-scoped and cannot have the namespaced policy as owner, so nothing is
garbage collected for nvpols. The controller must delete the generated pair itself:

- **Policy deleted**: `reconcile` gets NotFound from the lister and calls `deleteGeneratedVAP`.
- **Generation toggle off**: the pair is deleted and `status.generated` is reset to false. Read the toggle from the
  reconcile `ctx`, and do this before any early return.
- **Policy can no longer be generated** (no match constraints, only cluster-scoped rules, autogen, exceptions that
  cannot be converted): `handleVAPGeneration` takes the existing skip path, which deletes the pair and records
  `Generated=false` with the reason.
- **Controller outage**: generated objects carry the `policies.kyverno.io/source-policy-namespace` / `-name`
  annotations. On startup the VAP and binding informers list them, `enqueueVAP` / `enqueueVAPbinding` requeue the
  source policy, and the cases above remove leftovers. Keep those handlers checking the annotations before owner
  references.
- **No VAP API**: when the VAP or binding lister is nil, the nvpol branch returns without doing anything.

## Event dispatch that must include nvpols

- Policy exceptions reference policies by name and kind only, so a ref to `NamespacedValidatingPolicy` requeues every
  nvpol with that name (matching the engine). On update, requeue the policies of both the old and the new exception
  so a removed reference regenerates the VAP without the exception.
- Generated VAP and binding events requeue their source nvpol through the annotations.

Use `admissionpolicy.ValidatingPolicyVAPName` for generated names everywhere; never build `nvpol-...` names by hand.
