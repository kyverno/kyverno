## Description

This is a migrated test from e2e. It checks that a simple JSON patch `replace` operation works with a variable from AdmissionReview as a component of the `value` field.

## Expected Behavior

An Ingress's first rule should have the value of the `host` field appended to it `mycompany.com`. If this value has been replaced properly, the test passes. If not, the test fails.

## Reference Issue(s)

N/A