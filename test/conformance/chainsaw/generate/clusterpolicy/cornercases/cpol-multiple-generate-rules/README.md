## Description

This test checks when there are multiple generate rules in a policy, a single downstream per rule is created.

## Expected Behavior

Expect a single downstream (generated) resource to be created for each generate rule.
There should be a single configmap and single secret generated in the `default` namespace after the trigger ingress object is created.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/10587