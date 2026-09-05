# Test Case for Issue #11519

## Issue Description

When a test specifies that a resource should fail validation (result: fail) but the policy actually passes validation on that resource, Kyverno CLI was incorrectly reporting the test as passed.

This test case verifies that:
1. Resources that comply with the policy are correctly marked as `pass`
2. Resources that violate the policy are correctly marked as `fail`
3. The test results accurately reflect both pass and fail expectations

## Test Scenario

### Resources
- `deployment-pass.yaml`: A deployment with `allowPrivilegeEscalation: false` (complies with policy)
- `deployment-fail.yaml`: A deployment with `allowPrivilegeEscalation: true` (violates policy)

### Policy
- `policy.yaml`: Disallow Privilege Escalation (requires `allowPrivilegeEscalation: false`)

### Expected Results
- `deployment-pass` should PASS validation
- `deployment-fail` should FAIL validation

## Running the Test

```bash
cd test-case-11519
kyverno test .
```

Expected output should show:
- Test 1: deployment-pass → Result: Pass ✓
- Test 2: deployment-fail → Result: Fail ✓

If both tests show "Pass", the bug is present.

## Related Issue
- https://github.com/kyverno/kyverno/issues/11519
