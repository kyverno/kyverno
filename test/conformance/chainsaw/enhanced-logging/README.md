# Enhanced Logging Chainsaw Test

## Description
This test verifies that the enhanced logging feature for 'any' block failures works correctly in a real Kubernetes environment.

## Test Flow
1. **Create Policy**: Apply a policy with multiple 'any' blocks that will fail
2. **Test Enhanced Logging**: Apply a pod that violates the policy conditions
3. **Verify**: The pod should be rejected due to policy violations
4. **Check Logs**: Enhanced log messages should appear in Kyverno logs

## Expected Enhanced Logs
When the test runs, you should see enhanced log messages in the Kyverno controller logs:

```bash
# Check Kyverno logs for enhanced messages
kubectl logs -n kyverno-system deployment/kyverno | grep "no condition passed"
```

Expected output:
```
no condition passed for 'any' block for index '0' at 'precondition'
no condition passed for 'any' block for index '0' at 'deny condition'
```

## Running the Test
```bash
# Run with chainsaw
chainsaw test test/conformance/chainsaw/enhanced-logging/

# Or run all conformance tests
make test-conformance
```

## Verification
The test verifies:
1. Policy is applied successfully
2. Failing pod is rejected (as expected)
3. Enhanced logging provides better debugging context

The enhanced logging improvement is primarily visible in the server logs, providing developers with better debugging information when 'any' block conditions fail.
