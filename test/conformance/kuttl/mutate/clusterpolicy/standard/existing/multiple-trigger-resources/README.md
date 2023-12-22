## Description

This test makes sure that the generated updaterequest is deleted after applying the mutation.

## Expected Behavior

The target resource `pod` is mutated and all updaterequests are deleted.

## Steps

### Test Steps

1. Create a namespace.
2. Create two configmaps `test-org-1` and `test-org-2` i.e. the trigger resources.
3. Create a pod i.e. the target resource.
4. Create a policy that has `mutateExistingOnPolicyUpdate` set to true.
5. Two update requests are generated for both configmaps, one of which has a `failure` status. It is expected that both URs got deleted.
