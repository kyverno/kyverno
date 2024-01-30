## Description

This test checks to ensure that modification of a source (upstream) resource used by a Policy (Namespaced) generate rule, clone declaration, with sync disabled, does NOT result in those modifications being synced to the downstream resource.

## Expected Behavior

The source resource, once modified, should not cause any cloned (downstream) resources to be changed. If the downstream resource remains as-is, the test passes. If it is anything else other than how it looked when originally created, the test fails.

## Reference Issue(s)

N/A
