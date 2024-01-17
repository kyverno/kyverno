## Description

This test creates a policy to verify manifests signatures.
The policy specifies that two signatures are expected to be valid.

## Expected Behavior

Resource with no signature should be rejected.
Resource with one signature should be rejected.
Resource with two signatures should be accepted.
