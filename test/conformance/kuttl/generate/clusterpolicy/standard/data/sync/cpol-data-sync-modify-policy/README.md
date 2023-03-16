## Description

This test verifies the synchronize behavior of generated data resource, if the data pattern is modified in the policy rule, the changes should be synchronized to the downstream generated resource.

## Expected Behavior

This test ensures that update of the generate data rule gets synchronized to the downstream generated resource, otherwise the test fails. 

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/4222