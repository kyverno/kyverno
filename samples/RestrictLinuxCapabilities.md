# Restrict Linux capabilities

Linux divides the privileges traditionally associated with superuser into distinct units, known as capabilities, which can be independently enabled or disabled by listing them in `securityContext.capabilites`. A best practice is to limit the allowed capabilities to a minimal required set for each pod.

## Additional Information

* [List of linux capabilities](https://github.com/torvalds/linux/blob/master/include/uapi/linux/capability.h)


## Policy YAML

[restrict_capabilities.yaml](more/restrict_capabilities.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: restrict-capabilities
spec:
  rules:
  - name: validate-container-capablities
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Allow select linux capabilities"
      pattern:
        spec:
          containers:
          - securityContext:
              capabilities:
                add: ["NET_ADMIN"]

````

