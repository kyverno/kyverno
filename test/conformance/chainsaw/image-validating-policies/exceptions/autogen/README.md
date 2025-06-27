## Description

The policy contains autogen rules for cronjobs, deployments and statefulsets.
The policy enforces tighter security context by disallowing privilege escalation for containers.
The exception helps to bypass the policy for resources marked as "skipped" which are configured to violate the policy.

## Expected Behavior

The good deployment should pass regardless as it satisfies the requirement of the policy.
The bad deployment should be blocked as per the policy.
The skipped deployment should get skipped due to enforcement of the policyexception.
