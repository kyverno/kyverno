# Disallow use of bind mounts (`hostPath` volumes)

The volume of type `hostPath` allows pods to use host bind mounts (i.e. directories and volumes mounted to a host path) in containers. Using host resources can be used to access shared data or escalate priviliges. Also, this couples pods to a specific host and data persisted in the `hostPath` volume is coupled to the life of the node leading to potential pod scheduling failures. It is highly recommeded that applications are designed to be decoupled from the underlying infrstructure (in this case, nodes).

## Policy YAML 

[disallow_bind_mounts.yaml](best_practices/disallow_bind_mounts.yaml) 

````yaml
apiVersion: "kyverno.io/v1alpha1"
kind: "ClusterPolicy"
metadata: 
  name: "disallow-bind-mounts"
  annotations:
    policies.kyverno.io/category: Data Protection
    policies.kyverno.io/description: The volume of type `hostPath` allows pods to use host bind mounts (i.e. directories and volumes mounted to a host path) in containers. Using host resources can be used to access shared data or escalate priviliges. Also, this couples pods to a specific host and data persisted in the `hostPath` volume is coupled to the life of the node leading to potential pod scheduling failures. It is highly recommeded that applications are designed to be decoupled from the underlying infrstructure (in this case, nodes).

spec: 
  rules: 
  - name: "validate-hostPath"
    match: 
      resources: 
        kinds: 
        - "Pod"
    validate: 
      message: "Host path volumes are not allowed"
      pattern: 
        spec: 
          volumes: 
          - X(hostPath): null
````
