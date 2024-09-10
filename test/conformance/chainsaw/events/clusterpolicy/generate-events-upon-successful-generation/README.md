## Description

This test creates a generate policy, and the trigger resource (namespace).
Two events are generated:
1. An event for the policy to indicate that a new resource is generated.
2. An event for the generated resource itself.

## Steps

1. Patch `kyverno` configmap in `kyverno` namespace with `generateSuccessEvents` set to `true`
2. Create a generate policy
   Assert the policy becomes ready
3. Create the namespace.
4. An event is created for the policy with message "resource generated"
   An event is created for the generated resource.
