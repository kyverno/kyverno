# Assign Linux capabilities

Linux divides the privileges traditionally associated with superuser into distinct units, known as capabilities, which can be independently enabled or disabled by listing them in `securityContext.capabilites`. 

## Policy YAML

[policy_validate_container_capabilities.yaml](more/policy_validate_container_capabilities.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-container-capablities
spec:
  rules:
  - name: validate-container-capablities
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Allow certain linux capability"
      pattern:
        spec:
          containers:
          - securityContext:
              capabilities:
                add: ["NET_ADMIN"]

````

## Additional Information

* [List of linux capabilities](https://github.com/torvalds/linux/blob/master/include/uapi/linux/capability.h)
