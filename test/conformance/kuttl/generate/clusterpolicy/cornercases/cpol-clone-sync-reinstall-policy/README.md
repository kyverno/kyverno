## Description

This is a corner case test to ensure a generate data rule can be triggered on the deletion of the trigger resource.

## Expected Behavior

1. when the trigger is created, the corresponding downstream target secret should be generated
2. delete the policy, update the source, then re-install the policy with generateExisting=true, the change should be synced to the downstream target
3. update the source again, the change should be synced to the downstream target

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6398