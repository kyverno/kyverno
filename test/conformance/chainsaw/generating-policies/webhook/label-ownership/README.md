This test verifies generation-label ownership through the admission webhook used
by a `GeneratingPolicy`. It runs with the default
`features.protectManagedResources.enabled=false` setting.

A controller-created ConfigMap must carry the policy and actual trigger identity.
An ordinary service account can edit the downstream while preserving those
labels, but cannot create, add, change, or remove generation routing labels.
User-owned clone-source markers and standalone managed-by labels remain editable.
The policy does not synchronize downstream edits.

Run from the repository root against a test cluster with Kyverno installed:

```sh
chainsaw test --test-dir test/conformance/chainsaw/generating-policies/webhook/label-ownership
```
