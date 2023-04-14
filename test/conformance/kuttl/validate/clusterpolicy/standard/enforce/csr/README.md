## Description

This test mainly verifies that the JMESPath path for x509decode works for CSR does work properly.

## Expected Behavior

1. A policy is created to check Certificate Signing Requests and a policy that adds labels to the CSR.
2. A CSR Resource is created and it is verified that it has the same labels.

## Reference Issue(s)

5858