## Description

This tests checks that if namespace labels used in a policy are not present the resource is NOT created.
If the expected labels are defaulted in the policy the resource creation should work fine.

## Expected Behavior

The first part of the test checks that the resource fails to create if namespace labels are not present.
Then the policy is updated to use default values when namespace labels are missing, then the resource should be created without issue.

## Reference Issue(s)

5136
