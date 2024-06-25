## Description

This test ensures the variable `target` is allowed in a mutateExisting rule, except resource's spec definition under `mutate.targets`.

## Expected Behavior

The good policy should be allowed to create while the bad policy that contains `target.metadata.annotations.dns` cannot be created as it's invalid.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7379