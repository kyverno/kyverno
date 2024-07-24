## Description

This test ensures that a policy with two rules with different modes is applied correctly on resources and reports are successfully created.

## Expected Behavior

1. Create a policy that has two rules:
    - The first rule is `require-ns-purpose-label` in the `Enforce` mode that requires the `purpose` label to be set on namespaces.
    - The second rule is `require-ns-env-label` in the `Audit` mode that requires the `environment` field to be set on namespaces.

2. Create a `good-ns-1` namespace that has the `purpose` label. It is expected that the namespace will be created successfully.

3. Create a `good-ns-2` namespace that has both the `purpose` and `environment` labels. It is expected that the namespace will be created successfully.

4. Create a `bad-ns-1` namespace that doesn't have the `purpose` label. It is expected that the namespace will be blocked with a message reporting the violation of the `require-ns-purpose-label` rule.

5. Create a `bad-ns-2` namespace that doesn't have any labels. It is expected that the namespace will be blocked with messages reporting the violations of both rules.

6. Two ClusterPolicyReports will be created for each of the `good-ns-1` and `good-ns-2` namespaces.

## Reference Issue(s)

#10682
