## Description

This test ensures that the labels on reports are distributed correctly, preventing label length from exceeding Kubernetes' 63-character limit.

## Expected Behavior

The test should successfully generate both the EphemeralReport and PolicyReport resources, ensuring that the labels are appropriately handled and do not violate the character length constraints.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/11547