## Description

This test makes sure that the report is generated correctly when multiple exceptions are created for the same policy.

## Expected Behavior

1. Create a pod with two init containers. The first init container should have the `NET_ADMIN` and `NET_RAW` capabilities, and the second init container should have the `SYS_TIME` capability.

2. Create a policy that applies the baseline profile.

3. Create two exceptions for the init containters as follows:
   - The first exception `init1-exception-baseline` allows the values of `NET_ADMIN` and `NET_RAW` capabilities in the init containers. 
   - The second exception `init2-exception-baseline` allows the values of `SYS_TIME` capabilities in the init containers.

4. It is expected that a policy report is generated with a `skip` result.

5. Delete the first exception.

6. It is expected that a policy report is updated with a `fail` result since the first init container violates the policy and it isn't excluded by the second exception.



## Reference Issue(s)

#10580
