# Diallow privileged containers

Privileged containers are defined as any container where the container uid 0 is mapped to the host’s uid 0. A process within a privileged container can get unrestricted host access. With `securityContext.allowPrivilegeEscalation` enabled, a process can gain privileges from its parent.

To disallow privileged containers and privilege escalation, run pod containers with `securityContext.privileged` set to `false` and `securityContext.allowPrivilegeEscalation` set to `false`.

## Policy YAML 

[disallow_privileged.yaml](best_practices/disallow_privileged.yaml)

````yaml
apiVersion: kyverno.io/v1
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
      pattern:
        spec:
          containers:
          - =(securityContext):
              # https://github.com/kubernetes/api/blob/7dc09db16fb8ff2eee16c65dc066c85ab3abb7ce/core/v1/types.go#L5707-L5711
              # k8s default to false
              =(privileged): false
  - name: validate-allowPrivilegeEscalation
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Privileged mode is not allowed. Set allowPrivilegeEscalation to false"
      pattern:
        spec:
          containers:
          - securityContext:
              # https://github.com/kubernetes/api/blob/7dc09db16fb8ff2eee16c65dc066c85ab3abb7ce/core/v1/types.go#L5754
              allowPrivilegeEscalation: false
````
