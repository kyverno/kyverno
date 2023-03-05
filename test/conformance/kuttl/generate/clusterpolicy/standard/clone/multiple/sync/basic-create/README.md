## Description

This is a basic creation test of the "clone multiple" feature that ensures resources are created as expected by selecting the sources based upon label.

## Expected Behavior

If the `citrine` Namespace receives a Secret named `opal-secret` and a ConfigMap named `opal-cm`, the test passes. If it either does not receive one of these or it additionally receives a Secret named `forbidden`, the test fails.

## Reference Issue(s)

N/A