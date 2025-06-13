## Description

This test create two policies that uses CEL expressions:
- one that denies `Deployment/scale` in Audit mode
- one that denies `StatefulSet/scale` in Enforce mode

## Test Steps
1. Create a StatefulSet and a Deployment.
2. Attempt to scale the StatefulSet (expect failure).
3. Scale the Deployment (expect success).

## Purpose
This test ensures Kyverno correctly enforces CEL-based policies on subresources (such as scale) and that Audit/Enforce modes behave as expected.
