## Description

This is a basic creation test for a "generate existing" policy. It checks that the basic functionality works whereby installation of the policy causes correct evaluation of the match block resulting in generation of resources in only the matching result.

## Expected Behavior

If only the `red-ns` Namespace receives a generated NetworkPolicy, the test passes. If either it does not or `green-ns` or `winter-ns` receive NetworkPolicies, the test fails.

## Reference Issue(s)

N/A
