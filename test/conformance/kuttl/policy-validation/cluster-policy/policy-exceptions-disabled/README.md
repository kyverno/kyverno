## Description

This test is attempting to create a resource with the label "app: my-test-app", which would typically violate the policy defined. However, there is a policy exception defined for resources with the same label, which should bypass the policy. However, the policy exception for the label "app: my-test-app" has been explicitly disabled, meaning that the resource should still fail to be created despite being a matching exception.

## Expected Behavior

Resources should be rejected.