# Require a known ingress class

It can be useful to restrict Ingress resources to use a known ingress class that are allowed in the cluster.

You can customize this policy to allow ingress classes that are configured in the cluster.

## Policy YAML 

[known_ingress.yaml](best_practices/known_ingress.yaml) 

````yaml
apiVersion : kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: known-ingress
  annotations:
    policies.kyverno.io/category: Ingress
    policies.kyverno.io/description: 
spec:
  rules:
  - name: known-ingress
    match:
      resources:
        kinds:
        - Ingress
    validate:
      message: "Unknown ingress class"
      pattern:
        metadata:
          annotations:
            kubernetes.io/ingress.class: "F5 | nginx"
````
