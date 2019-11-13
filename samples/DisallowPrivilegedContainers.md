# Diallow privileged containers

Privileged containers are defined as any container where the container uid 0 is mapped to the host’s uid 0. A process within a privileged container can get unrestricted host access. With `securityContext.allowPrivilegeEscalation` enabled, a process can gain privileges from its parent.

To disallow privileged containers and privilege escalation, run pod containers with `securityContext.privileged` set to `false` and `securityContext.allowPrivilegeEscalation` set to `false`.

## Policy YAML 

[disallow_privileged.yaml](best_practices/disallow_privileged.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: disallow-privileged
spec:
  rules:
  - name: validate-privileged
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Privileged mode is not allowed. Set privileged to false"
      anyPattern:
      - spec:
          securityContext:
            privileged: false
      - spec:
          containers:
          - name: "*"
            securityContext:
              privileged: false
  - name: validate-allowPrivilegeEscalation
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Privileged mode is not allowed. Set allowPrivilegeEscalation to false"
      anyPattern:
      - spec:
          securityContext:
            allowPrivilegeEscalation: false
      - spec:
          containers:
          - name: "*"
            securityContext:
              allowPrivilegeEscalation: false
````
