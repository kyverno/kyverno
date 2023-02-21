## Description

This is a test to ensure a resource name with over 63 characters is properly truncated when used in an updaterequest and target resource contains the expected kyverno.io/generated-by-name label. 

## Expected Behavior

It ensures that a secret is created with the `kyverno.io/generated-by-name` label set to the truncated tigger resource name. 

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/4675