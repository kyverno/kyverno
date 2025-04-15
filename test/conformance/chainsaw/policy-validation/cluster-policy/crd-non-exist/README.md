## Description

This test validates a policy can be successfully created if the matching custom resource (its CRD) has not been registered yet.

## Expected Behavior

1. The policy creation is allowed with the status ready=false
2. The policy ready status becomes true after create the CRD.

## Reference Issues

https://github.com/kyverno/kyverno/issues/11701
