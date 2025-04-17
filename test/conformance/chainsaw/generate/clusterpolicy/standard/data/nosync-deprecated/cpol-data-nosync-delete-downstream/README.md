# Title

This is a generate test to ensure deleting a generate policy using a data declaration with sync enabled deletes the downstream ConfigMap when matching a new Namespace.

## Expected Behavior

If the generated (downstream) resource is not recreated, the test passes. If it is recreated from the definition in the rule, the test fails.

## Reference Issue(s)

N/A