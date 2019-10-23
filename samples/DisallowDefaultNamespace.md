# Disallow use of default namespace

Namespaces are a way to segment and isolate cluster resources across multiple users. When multiple users or teams are sharing a single cluster, it is recommended to isolate different workloads and restrict use of the default namespace.

## Policy YAML 

[disallow_default_namespace.yaml](best_practices/disallow_default_namespace.yaml) 

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-namespace
spec:
  rules:
  - name: check-default-namespace
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Using 'default' namespace is restricted"
      pattern:
        metadata:
          namespace: "!default"
  - name: check-namespace-exist
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "A namespace is required"
      pattern:
        metadata:
          namespace: "?*"
````
