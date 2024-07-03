## Description

This test validates that an existing ConfigMap can't be updated with a new key that results in violation of a policy.

## Expected Behavior

The existing ConfigMap isn't patched and policy violation is reported.

## Steps

### Test Steps

1. Create a `Policy` that denies only permits combination of two particular keys together.
2. Create a `ConfigMap` that contains one of the keys.
3. Try to patch the `ConfigMap` with a new key that is not permitted by the policy.
4. Verify that the `ConfigMap` is not patched and policy violation is reported.
5. Delete the `Policy` and `ConfigMap`.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/3253