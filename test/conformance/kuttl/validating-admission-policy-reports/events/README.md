# Title

This test checks for generated events when applying ValidatingAdmissionPolicies.

## Expected Behavior


This test creates a deployment with 4 replicas that violates the policy. It verifies policy violation events generation for the ValidatingAdmissionPolicy and the Deployment.

## Reference Issues

https://github.com/kyverno/kyverno/issues/8781
