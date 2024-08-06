## Description

This test patches `kyverno` configmap in `kyverno` namespace with `generateSuccessEvents` set to `false`
Then it creates a policy and a resource.
The resource is expected to be accepted.
A `PolicyApplied` event with message `ConfigMap default/foo: pass` from object `policy/require-labels` should not be created as `generateSuccessEvents` config is set to `false`


## Steps

1. Patch `kyverno` configmap in `kyverno` namespace with `generateSuccessEvents` set to `false`
2. Create a policy
3. Assert the policy becomes ready
4. Create a resource
5. Assert there is no `PolicyApplied` event with message `ConfigMap default/foo: pass` is created via script
6. Exit the script with code `1` if it returns an error
