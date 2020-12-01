# Require Pods Drop All Capabilities

Containers may optionally ask for specific Linux capabilities without requiring root on the node. As a security best practice, containers should only specify exactly which capabilities they need. This starts with dropping all capabilities and only selectively adding ones back.

This example policy requires that all containers drop all capabilities.

## More information

* [Set Capabilities for a Container](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-capabilities-for-a-container)

## Policy YAML

[require_drop_all.yaml](more/require_drop_all.yaml)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: drop-all-capabilities
spec:
  validationFailureAction: audit
  rules:
  - name: drop-all-containers
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Drop all must be defined for every container in the Pod."
      pattern:
        spec:
          containers:
          - securityContext:
              capabilities:
                drop: ["ALL"]
  - name: drop-all-initcontainers
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Drop all must be defined for every container in the Pod."
      pattern:
        spec:
          initContainers:
          - securityContext:
              capabilities:
                drop: ["ALL"]
```
