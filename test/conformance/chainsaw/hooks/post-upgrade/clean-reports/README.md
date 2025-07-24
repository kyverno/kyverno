## Description

This test creates a policy with two rules.
The rules are respectively namespace and cluster scoped.
The test checks if, after a helm upgrade of kyverno, the post-hook clean-reports job runs correctly and deletes all the ClusterPolicyReports/PolicyReports objects.

## Related issue

No issue was found.