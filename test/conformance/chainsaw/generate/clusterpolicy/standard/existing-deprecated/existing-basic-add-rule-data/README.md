## Description

This is a basic creation test for a "generate existing" policy. It checks that the basic functionality works whereby creation of a new rule causes correct evaluation of the match block resulting in generation of resources in only the matching result.

## Expected Behavior

If both `blue-ns` and `yellow-ns` Namespaces receive a generated NetworkPolicy, and `summer-ns` does not receive a NetworkPolicies, the test passes. Otherwise the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6471
