## Description

This test verifies multiple image attestations using notary signatures

## Expected Behavior

This test creates a cluster policy.
When a pod is created with the image reference and the signature on multiple attestations matches, the pod creation is successful

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/9456