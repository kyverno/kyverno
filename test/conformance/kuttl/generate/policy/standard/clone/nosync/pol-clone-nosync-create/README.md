## Description

This test checks the basic creation behavior of a generate rule in a Policy (Namespaced) using a clone declaration with synchronize disabled.

## Expected Behavior

A resource should be generated via clone in the same Namespace as where the Policy is created. If the resource is created, the test passes. If the resource is not, the test fails.

## Reference Issue(s)

N/A
