## Description

This is a corner case test to ensure that "pods/ephemeralcontainers" are added in the match block of the ValidatingAdmissionPolicy.

## Expected Behavior

The test should pass if the "pods/ephemeralcontainers" are added in the match block of the ValidatingAdmissionPolicy. If not, the test fails. Moreover, a Pod is created and the policy should block the use of ephemeral containers.
