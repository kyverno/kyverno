## Description

This test verifies a variable definition is not evaluated until the condition is used

## Expected Behavior

The policy should not cause an error if the first condition (any) passes. The policy should cause an error if the first condition (all) fails.

## Reference Issues

https://github.com/kyverno/kyverno/issues/7211