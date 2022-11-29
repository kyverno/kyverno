## Description

This test verifies the synchronized behavior of generated resource. If the generated resource is updated, then the generated resource should revert to the source resource.

## Expected Behavior

This test ensures that any update in generated resource(Secret: myclonedsecret) should result in reverting the generated resource to the source resource, otherwise the test fails.
The source resource is identified through the policy which created the generated resource. 

## Reference Issue(s)

#5100
