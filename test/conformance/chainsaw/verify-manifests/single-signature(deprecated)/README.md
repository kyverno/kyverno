## Description

This test creates a policy to verify manifests signatures.
The policy specifies that at least one signature is expected to be valid.

## Expected Behavior

Resource with no signature should be rejected.
Resource with one signature should be accepted.
Resource with two signatures should be accepted.
