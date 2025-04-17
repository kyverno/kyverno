## Description

This test ensures that a policy with two rules; one of which doesn't specify the `ValidationFailureAction` field, and the other specifies the `ValidationFailureAction` field, works as expected. The rule which don't specify the action should use the default action in `spec.ValidationFailureAction`.

## Expected Behavior

1. Create a policy that has two rules:
    - The first rule is `require-ns-purpose-label` in the `Enforce` mode that requires the `purpose` label to be set on namespaces.
    - The second rule is `require-ns-env-label`  requires the `environment` field to be set on namespaces and doesn't specify the `ValidationFailureAction` field.

2. Create a `good-ns-1` namespace that has the `purpose` label. It is expected that the namespace will be created successfully.

3. Create a `good-ns-2` namespace that has both the `purpose` and `environment` labels. It is expected that the namespace will be created successfully.

4. Create a `bad-ns-1` namespace that doesn't have the `purpose` label. It is expected that the namespace will be blocked with a message reporting the violation of the `require-ns-purpose-label` rule.

5. Create a `bad-ns-2` namespace that doesn't have any labels. It is expected that the namespace will be blocked with messages reporting the violations of both rules.

## Reference Issue(s)

#10682
