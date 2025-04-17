## Description

This test ensures that reports are generated as a result of background scanning when a policy with two rules with different modes is applied on resources.

## Expected Behavior

1. Create a `good-ns-1` namespace that has the `purpose` label.

2. Create a `good-ns-2` namespace that has both the `purpose` and `environment` labels.

3. Create a `bad-ns-1` namespace that doesn't have the `purpose` label.

4. Create a `bad-ns-2` namespace that doesn't have any labels.

5. Create a policy that has two rules:
    - The first rule is `require-ns-purpose-label` in the `Enforce` mode that requires the `purpose` label to be set on namespaces.
    - The second rule is `require-ns-env-label` in the `Audit` mode that requires the `environment` field to be set on namespaces.

6. Four ClusterPolicyReports will be created for each of the `good-ns-1`, `good-ns-2`, `bad-ns-1`, and `bad-ns-2` namespaces.

## Reference Issue(s)

#10682
