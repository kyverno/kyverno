## Description

This test creates an exception for the init containers to set the `runAsNonRoot` to false

## Expected Behavior

1. Create a policy that applies the restricted profile.

2. Create an exception for the init containters to set the `runAsNonRoot` to false.

3. Create a pod with the following characteristics:
   - The pod has an init container that sets the `runAsNonRoot` field to `false`.
   - The pod has a container that doesn't set the `runAsNonRoot` field.

   It is expected that the pod will be blocked with a message reporting the violation of the container. The init container is already excluded by the exception.

3. Create a pod with the following characteristics:
    - The pod has an init container that sets the `runAsNonRoot` field to `true`.
    - The pod has a container that doesn't set the `runAsNonRoot` field.
    
    It is expected that the pod will be blocked with a message reporting the violation of the container.

4. Create a pod with the following characteristics:
    - The pod has an init container that sets the `runAsNonRoot` field to `false`.
    - The pod has a container that doesn't set the `runAsNonRoot` field.
    - `runAsNonRoot` is set to `true` in the pod spec.

    It is expected that the pod will be created successfully.

## Reference Issue(s)

#10581
