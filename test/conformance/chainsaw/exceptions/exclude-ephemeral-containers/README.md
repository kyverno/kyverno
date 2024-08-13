## Description

This test makes sure that policy exception matches `Pod/epehemeralcontainers` resource by default in case it matches Pods.

## Expected Behavior
1. Create a policy that matches Pods and restrict setting `runAsNonRoot` to `true`.

2. Create an exception that excludes Pods from the policy.

3. Create a Pod that violates the policy. It is expected that the Pod will be created successfully as it matches the exception.

4. Run `kubectl debug` command to attach to the Pod. It is expected that the command will run successfully since exceptions match `Pod/ephemeralcontainers` resource by default.

## Reference Issue(s)

#9484
