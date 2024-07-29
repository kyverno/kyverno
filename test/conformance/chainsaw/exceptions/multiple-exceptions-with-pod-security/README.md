## Description

This test creates two policy exceptions that match the same policy. It is expected that the pod that satisfies both exceptions will be created successfully.

## Expected Behavior

1. Create a policy that applies the baseline profile.

2. Create two exceptions for the init containters as follows:
   - The first exception `init1-exception-baseline` allows the values of `NET_ADMIN` and `NET_RAW` capabilities in the init containers. 
   - The second exception `init2-exception-baseline` allows the values of `SYS_TIME` capabilities in the init containers. 

3. Create a pod with two init containers. The first init container should have the `NET_ADMIN` and `NET_RAW` capabilities, and the second init container should have the `SYS_TIME` capability. It is expected that the pod will be created successfully as it matches both exceptions.


## Reference Issue(s)

#10580
