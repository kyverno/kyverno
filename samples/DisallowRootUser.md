# Run as non-root user

By default, all processes in a container run as the root user (uid 0). To prevent potential compromise of container hosts, specify a non-root and least privileged user ID when building the container image and require that application containers run as non root users i.e. set `runAsNonRoot` to `true`.

## Additional Information

* [Pod Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)

## Policy YAML 

[disallow_root_user.yaml](best_practices/disallow_root_user.yaml) 

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-root-user
  annotations:
spec:
  rules:
  - name: validate-runAsNonRoot
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Root user is not allowed. Set runAsNonRoot to true"
      anyPattern:
      - spec:
          securityContext:
            runAsNonRoot: true
      - spec:
          containers:
          - name: "*"
            securityContext:
              runAsNonRoot: true
````
