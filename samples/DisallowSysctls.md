# Disallow changes to kernel parameters

The Sysctl interface allows modifications to kernel parameters at runtime. In a Kubernetes pod these parameters can be specified under `securityContext.sysctls`. Kernel parameter modifications can be used for exploits and should be restricted.

## Additional Information

* [List of supported namespaced sysctl interfaces](https://kubernetes.io/docs/tasks/administer-cluster/sysctl-cluster/) 


## Policy YAML

[disallow_sysctls.yaml](best_practices/disallow_sysctls.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: disallow-sysctls
spec:
  rules:
  - name: validate-sysctls
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Changes to kernel paramaters are not allowed"
      pattern:
        spec:
          securityContext:
            X(sysctls): null
````
