## Description

This test mainly verifies that the JMESPath path for x509decode works for CSR does work properly.

## Expected Behavior

1. A policy is created to check Certificate Signing Requests and denies all requests
2. A CSR Resource is created, which will fail.
3. The response should have username in it which is a part of the decoded CSR, this shows that the CSR was properly decoded.

## Reference Issue(s)

5858