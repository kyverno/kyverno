## Description

This test performs a check to ensure that a "good" Policy, one in which a user may attempt to in-Namespace generate a resource, is allowed to be created.

This test is basically identical to a similar one in which sync is disabled and the results should be the same. In this test, the setting of `sync` is irrelevant yet is tested here for completeness.

## Expected Behavior

"good" (valid) Policy should be successfully created. If the creations is blocked, the test failed. If any creation is allowed, the test succeeds.

## Reference Issue(s)

5099
