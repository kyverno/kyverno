# Disallow use of default namespace

Kubernetes namespaces provide a way to segment and isolate cluster resources across multiple applictaions and users. It is recommended that each workload be isolated in its own namespace and that use of the default namespace be not allowed.

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
