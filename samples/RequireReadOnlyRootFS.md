# Require read-only root filesystem

A read-only root file system helps to enforce an immutable infrastructure strategy; the container only needs to write on mounted volumes that can persist state even if the container exits. An immutable root filesystem can also prevent malicious binaries from writing to the host system.

## Policy YAML 

[require_ro_rootfs.yaml](best_practices/require_ro_rootfs.yaml)


````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: require-ro-rootfs
spec:
  rules:
  - name: validate-readOnlyRootFilesystem
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Root filesystem must be read-only"
      pattern:
        spec:
          containers:
          - securityContext:
              readOnlyRootFilesystem: true
````
