# Disallow use of default namespace

Kubernetes namespaces are an optional feature that provide a way to segment and isolate cluster resources across multiple applications and users. As a best practice, workloads should be isolated with namespaces. Namespaces should be required and the default (empty) namespace should not be used.

## Policy YAML 

[disallow_default_namespace.yaml](best_practices/disallow_default_namespace.yaml) 

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: disallow-default-namespace
spec:
  rules:
  - name: validate-namespace
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Using 'default' namespace is not allowed"
      pattern:
        metadata:
          namespace: "!default"
  - name: require-namespace
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
