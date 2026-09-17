# Test Case for Issue #11519

## Issue Description

When a test specifies that a resource should fail validation (`result: fail`) but the policy actually passes validation on that resource, the Kyverno CLI was incorrectly reporting the test as passed.

This test case guards against that regression. Because the CLI currently reports `test-deploy-pass` as a failure, the manifest intentionally expects `fail` for that resource. Once the underlying behavior is fixed, the expectation for `test-deploy-pass` should be flipped back to `pass`.

## Test Scenario

### Resources
- `deployment-pass.yaml`: A Deployment with `allowPrivilegeEscalation: false` (complies with policy)
- `deployment-fail.yaml`: A Deployment with `allowPrivilegeEscalation: true` (violates policy)

### Policy
- `policy.yaml`: Disallow Privilege Escalation (requires `allowPrivilegeEscalation: false`)

### Expected Results
- `test-deploy-pass` is expected to report `fail` under the current CLI behavior
- `test-deploy-fail` is expected to report `fail`

## Running the Test

```bash
cd test-case-11519
kyverno test .
```

Expected output should show:
- Test 1: `test-deploy-pass` → Result: `Fail`
- Test 2: `test-deploy-fail` → Result: `Fail`

When the bug from #11519 is fixed, `test-deploy-pass` should start reporting `Pass` and the test expectation must be updated accordingly.

## Related Issue
- https://github.com/kyverno/kyverno/issues/11519
