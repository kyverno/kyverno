# Disallow use of host filesystem

The volume of type `hostpath` binds pods to a specific host, and data persisted in the volume is dependent on the life of the node. In a shared cluster, it is recommeded that applications are independent of hosts.

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
