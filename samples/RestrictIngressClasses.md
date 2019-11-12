# Restrict ingress classes

It can be useful to restrict Ingress resources to a set of known ingress classes that are allowed in the cluster. You can customize this policy to allow ingress classes that are configured in the cluster.

## Policy YAML 

[restrict_ingress_classes.yaml](more/restrict_ingress_classes.yaml) 

````yaml
apiVersion : kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: restrict-ingress-classes
spec:
  rules:
  - name: validate-ingress
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
