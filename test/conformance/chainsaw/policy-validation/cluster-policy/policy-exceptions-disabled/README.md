## Description

This test is attempting to create a resource with the label "app: my-test-app", which would typically violate the policy defined. However, there is a policy exception defined for resources with the same label, which should bypass the policy. Since the Policy Exception feature has not been enabled, the resource will be blocked by the policy instead of being allowed.

## Expected Behavior

The Pod should be blocked.