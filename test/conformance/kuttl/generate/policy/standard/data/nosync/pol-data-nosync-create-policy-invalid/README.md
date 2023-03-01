## Description

This test checks to ensure that a "bad" Policy (Namespaced) cannot be created which attempts to generate a resource into a different Namespace from that in which the Policy exists.

## Expected Behavior

If the Policy cannot be created, the test passes. If it is allowed to be created, the test fails.

## Reference Issue(s)

N/A