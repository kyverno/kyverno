## Description

This is a basic test for generate existing foreach clone policy, with preconditions and context variables. It checks that the basic functionality works whereby installation of the policy causes correct evaluation of the match and preconditions blocks.

## Expected Behavior

If only the `foreach-ns-1` Namespace receives a cloned Secret upon policy creation, the test passes. If either it does not or `foreach-ns-2` receives a cloned Secret, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/3542