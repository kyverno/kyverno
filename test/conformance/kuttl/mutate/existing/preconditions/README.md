## Description

This test creates pods and a policy to add a label to existing pods with per target preconditions based on the target annotations.

## Expected Behavior

Only the pod with `policy.lan/value: foo` annotation has the label `policy-applied: 'true'` added.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7174
