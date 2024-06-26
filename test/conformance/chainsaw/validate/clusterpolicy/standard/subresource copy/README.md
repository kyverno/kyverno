## Description

This test create two policies:
- one that denies `Deployment/scale` in Audit mode
- one that denies `StatefulSet/scale` in Enforce mode

It then create a statefulset and a deployment.

Finally it tries to create the statefulset and expects failure, the, scales the deployment and expects success.
