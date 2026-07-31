## Description

This test verifies that a generate policy does NOT create downstream resources
when the trigger resource's UID does not match any live resource.

This simulates the scenario where a Create admission request is rejected by the
API server (AlreadyExists), but Kyverno's webhooks have already processed the
request and created an UpdateRequest. Without the fix for issue #16566, the
background controller would fall back to the admission request bytes and
create downstream resources for a namespace that only existed in the rejected
request.

## Expected Behavior

- The live namespace (layer=operational) is NOT matched by the policy
  (which matches layer=business)
- The UpdateRequest has a phantom UID that doesn't match any live namespace
- The background controller marks the UR as Failed
- No downstream ConfigMap is created

## Reference Issue(s)

#16566
