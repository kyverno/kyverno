## Description

This test creates a policy that only allows a maximum of 3 containers inside a pod. It then creates an exception with a namespace selector that allows the exception to be applied to pods and deployments deployed in  namespaces with the label `env: production`.


## Expected Behavior

The deployment `skipped-deployment` in the namespace `production` is created successfully because it matches the exception's namespace selector. The deployment `bad-deployment` in the default namespace is blocked because it does not match the exception's namespace selector and it violates the policy.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/13941
