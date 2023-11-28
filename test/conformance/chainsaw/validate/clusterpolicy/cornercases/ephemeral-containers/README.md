## Description

This test ensures that Kyverno is able to perform basic validation functions against ephemeral containers.

## Expected Behavior

The initial Pod should be successfully created. An ephemeral container, added via the `kubectl debug` imperative command, should be allowed because it does not violate the policy. If the ephemeral container is added, the test passes. If the debug is blocked, the test fails.

## Reference Issue(s)

6943