# Require read-only root filesystem

A read-only root file system helps to enforce an immutable infrastructure strategy; the container only needs to write on mounted volumes that can persist state even if the container exits. An immutable root filesystem can also prevent malicious binaries from writing to the host system.

## Policy YAML 

[require_readonly_rootfilesystem.yaml](best_practices/require_readonly_rootfilesystem.yaml)


````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-readonly-rootfilesystem
spec:
  rules:
  - name: validate-readonly-rootfilesystem
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Container require read-only rootfilesystem"
      pattern:
        spec:
          containers:
          - securityContext:
              readOnlyRootFilesystem: true
````
