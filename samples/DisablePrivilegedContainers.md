# Disable privileged containers

Privileged containers are defined as any container where the container uid 0 is mapped to the host’s uid 0. A process within a privileged container can get unrestricted host access. With `securityContext.allowPrivilegeEscalation` enabled, a process can gain privileges from its parent.

To disallow privileged containers and the privilege escalation it is recommended to run pod containers with `securityContext.priveleged` set to `false` and `allowPrivilegeEscalation` set to `false`.

## Policy YAML 

[disallow_priviledged_priviligedescalation.yaml](best_practices/disallow_priviledged_priviligedescalation.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-deny-privileged-priviligedescalation
spec:
  rules:
  - name: deny-privileged-priviligedescalation
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Privileged mode is not allowed. Set allowPrivilegeEscalation and privileged to false"
      anyPattern:
      - spec:
          securityContext:
            allowPrivilegeEscalation: false
            privileged: false
      - spec:
          containers:
          - name: "*"
            securityContext:
              allowPrivilegeEscalation: false
              privileged: false    
````
