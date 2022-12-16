## Description

Verify image attestations with the given predicateType and attestors. The image has multiple signatures for different predicateTypes.

## Expected Behavior

Given another defined predicateType, the image's subject and issuer match as well as the attestation specified in the conditions block. The pod creation should pass.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/4847
