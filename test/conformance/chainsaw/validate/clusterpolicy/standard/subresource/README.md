## Description

This test creates two policies:
- one that denies `Deployment/scale` in Audit mode
- one that denies `StatefulSet/scale` in Enforce mode

It then creates a StatefulSet and a Deployment.

Finally, it tries to scale the StatefulSet and expects failure, then scales the Deployment and expects success.
