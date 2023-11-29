## Description

This test checks to ensure that when a standard generate policy with data type and sync enabled is used, deletion of the generated/downstream resource causes Kyverno to re-create the resource.

## Expected Behavior

If the resource is recreated, the test passes. If it is not, the test fails.

## Reference Issue(s)

N/A