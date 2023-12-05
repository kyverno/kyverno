## Description

This test performs two checks to ensure that a "bad" Policy, one in which a user may attempt to cross-Namespace generate a resource, is blocked from creation.

This test is basically identical to a similar one in which sync is disabled and the results should be the same. In this test, the setting of `sync` is irrelevant yet is tested here for completeness.

## Expected Behavior

"bad" (invalid) Policy should fail to be created. If all the creations are blocked, the test succeeds. If any creation is allowed, the test fails.

## Reference Issue(s)

5099
