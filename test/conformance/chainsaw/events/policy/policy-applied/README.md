## Description

This test creates a policy, and a resource.
A `PolicyApplied` event should be created.

## Steps

1. Patch `kyverno` configmap in `kyverno` namespace with `generateSuccessEvents` set to `true` 
2. Create a policy
3. Assert the policy becomes ready
4. Create a resource
5. Assert a `PolicyApplied` event is created
