## Description

This is a corner case test to ensure a generate clone rule can be triggered on the deletion of the trigger resource. It also ensures upgrades to 1.10 are successful for the same clone rule type.

## Expected Behavior

1. when the trigger is created, the corresponding downstream target secret should be generated
2. delete the policy, update the source, then re-install the policy with generateExisting=true, the change should be synced to the downstream target
3. update the source again, the change should be synced to the downstream target

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7170