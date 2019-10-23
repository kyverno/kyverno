# Disallow use of host filesystem

The volume of type `hostpath` allows pods to use host directories and volume mounted to a host path. This binds pods to a specific host, and data persisted in the volume is coupled to the life of the node. It is highly recommeded that applications are designed to be decoupled from the underlying infrstructure (in this case, nodes).

## Policy YAML 

[disallow_host_filesystem.yaml](best_practices/disallow_host_filesystem.yaml) 

````yaml
apiVersion: "kyverno.io/v1alpha1"
kind: "ClusterPolicy"
metadata: 
  name: "deny-use-of-host-fs"
spec: 
  rules: 
  - name: "deny-use-of-host-fs"
    match: 
      resources: 
        kinds: 
        - "Pod"
    validate: 
      message: "Host path is not allowed"
      pattern: 
        spec: 
          volumes: 
          - X(hostPath): null
````
