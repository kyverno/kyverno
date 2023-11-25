## Description

This test checks the Kyverno can generate a Role and RoleBinding from a clone-type generate rule. This test does NOT require additional privileges granted to the Kyverno ServiceAccount. Because this is a test which covers generation of security-related constructs which the API server has special logic to block if it detects a possible privilege escalation attack, it is being considered a corner case. This test was migrated from e2e.

## Expected Behavior

The Role and RoleBinding should be generated as per the clone declaration in the ClusterPolicy.

## Reference Issue(s)

N/A