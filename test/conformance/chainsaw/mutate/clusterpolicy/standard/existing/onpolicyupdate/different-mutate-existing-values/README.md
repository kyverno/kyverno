## Description

This test ensures that a mutate policy of two rules with different values of `mutateExistingOnPolicyUpdate` works as expected. 

## Expected Behavior

1. Create two Namespaces `staging-2` and `staging-3`.

2. Create two Secrets `test-secret-2` and `test-secret-3` in Namespaces `staging-2` and `staging-3` respectively.

3. Create two ConfigMaps `dictionary-2` and `dictionary-3` in Namespaces `staging-2` and `staging-3` respectively.

4. Create a ClusterPolicy with two mutate rules:
    - The first rule matches a ConfigMap named `dictionary-3` in Namespace `staging-3` and sets the value of `mutateExistingOnPolicyUpdate` to `false`. Its target is to mutate a Secret named `test-secret-3` in Namespace `staging-3`.

    - The second rule matches a ConfigMap named `dictionary-2` in Namespace `staging-2` and sets the value of `mutateExistingOnPolicyUpdate` to `false`. Its target is to mutate a Secret named `test-secret-2` in Namespace `staging-2`.

5. On policy creation, the Secret `test-secret-3` in Namespace `staging-3` should be mutated whereas the Secret `test-secret-2` in Namespace `staging-2` should not be mutated.

## Reference Issue(s)

N/A