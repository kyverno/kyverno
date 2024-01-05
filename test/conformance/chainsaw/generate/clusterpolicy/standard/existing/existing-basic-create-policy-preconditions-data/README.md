## Description

This is a basic creation test for a "generate existing" policy with preconditions. It checks that the basic functionality works whereby installation of the policy causes correct evaluation of the match and preconditions blocks.

## Expected Behavior

If only the `jupiter` Namespace receives a generated ConfigMap, the test passes. If either it does not or `venus` receives a ConfigMap, the test fails.

## Reference Issue(s)

N/A
