# Title

This test creates a deployment with Violation replicas.
It then generates policy violation events for scanning validating admission policies in reports controller.

## Expected Behavior


This test creates a deployment with 3 replicas that violates the policy. It verifies policy violation events generation for the ValidatingAdmissionPolicy and the Deployment.

## Reference Issues

https://github.com/kyverno/kyverno/issues/8781
