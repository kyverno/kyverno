## Description

This test ensures that cascading mutation (a combined mutation resulting from two or more rules which have a dependency) with strategic merge patches results in correct output.

## Expected Behavior

If the Cassandra Pod has labels `type=database` and `backup-needed="yes"` assigned, the test passes. If it is missing either one, the test fails.

## Reference Issue(s)

N/A