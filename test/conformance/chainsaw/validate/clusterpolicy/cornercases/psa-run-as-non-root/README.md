## Description

This test ensures that pods whose container don't set the `runAsNonRoot` field but init container sets the field to `false` are blocked by the `psa-run-as-non-root` policy with messages reporting both violations.

## Expected Behavior

1. Create a policy that applies the restricted profile.

2. Create a pod with the following characteristics:
   - The pod has an init container that sets the `runAsNonRoot` field to `false`.
   - The pod has a container that doesn't set the `runAsNonRoot` field.

   It is expected that the pod will be blocked with a message reporting both violations.

3. Create a pod with the following characteristics:
    - The pod has an init container that sets the `runAsNonRoot` field to `true`.
    - The pod has a container that doesn't set the `runAsNonRoot` field.
    
    It is expected that the pod will be blocked with a message reporting the violation of the container.

4. Create a pod with the following characteristics:
    - The pod has an init container that sets the `runAsNonRoot` field to `true`.
    - The pod has a container that doesn't set the `runAsNonRoot` field.
    - `runAsNonRoot` is set to `true` in the pod spec.

    It is expected that the pod will be created successfully.

## Reference Issue(s)

#10581
