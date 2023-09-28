# Title

This test creates a deployment with four replicas.
It then creates a validating admission policy that checks the replicas of the deployment.

## Expected Behavior

The deployment is created and a policy report is generated for it with a fail result.
