This test verifies generation-label ownership independently of a
`GeneratingPolicy` trigger webhook. It runs with the default
`features.protectManagedResources.enabled=false` setting.

A ConfigMap trigger generates a Secret in a different namespace, without the
trigger object-selector label. The controller-created Secret must carry the
policy and actual trigger identity.
An ordinary service account can edit the downstream while preserving those
labels, but cannot create, add, change, or remove generation routing labels.
User-owned clone-source markers and standalone managed-by labels remain editable.
The dedicated metadata webhook selects the policy association on either the old
or new object, so removing the association still reaches protection. Ordinary
resources without a policy association do not need this callback. The policy
does not synchronize downstream edits.

Run from the repository root against a test cluster with Kyverno installed:

```sh
chainsaw test --test-dir test/conformance/chainsaw/generating-policies/webhook/label-ownership
```
