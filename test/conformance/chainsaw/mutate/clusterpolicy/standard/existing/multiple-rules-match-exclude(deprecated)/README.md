## Description

This test ensures that match and exclude are applied to the incoming resource correctly, and only the matched rule gets applied.

## Expected Behavior

Both pod `nginx-a` and `nginx-b` has label `policy.lan/remove-flag: 'true'` added but not `policy.lan/apply-flag: "true"`.

## Steps

### Test Steps

1. Create `ClusterRole` that grants the proper permission to apply the mutateExisting policy.
2. Create `Namespace` and two pods in the namespace.
3. Create the `ClusterPolicy` with two mutate existing rules.
4. Remove the label on the `Namespace` to trigger the policy.
5. Verify that the desired label is added to both pods, and undesired label is not added.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7192
