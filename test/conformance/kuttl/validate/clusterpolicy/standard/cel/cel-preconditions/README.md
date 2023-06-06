## Description

This test validates the use of `rule.celPreconditions`. 
The policy will be applied on resources that matches the CEL Preconditions.

## Expected Behavior

The policy will be applied on `pod-fail` and since it violates the rule, it will be blocked.
The policy won't be applied on `pod-pass` because it doesn't match the CEL precondition. Therefore it will be created.
