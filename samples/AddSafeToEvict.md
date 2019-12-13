# Mutate pods with `emptyDir` and `hostPath` with `safe-to-evict`

The Kubernetes cluster autoscaler does not evict pods that use `hostPath` or `emptyDir` volumes. To allow eviction of these pods, the following annotation must be added to the pods:

````yaml
cluster-autoscaler.kubernetes.io/safe-to-evict: true
````

This policy matches and mutates pods with `emptyDir` and `hostPath` volumes, to add the `safe-to-evict` annotation if it is not specified.

## Policy YAML

[add_safe_to_evict_annotation.yaml](best_practices/add_safe_to_evict.yaml)

````yaml
apiVersion: "kyverno.io/v1"
kind: ClusterPolicy
metadata: 
  name: "add-safe-to-evict"
spec: 
  rules: 
  - name: "annotate-empty-dir"
    match: 
      resources: 
        kinds: 
        - "Pod"
    mutate: 
      overlay:
        metadata:
          annotations:
            +(cluster-autoscaler.kubernetes.io/safe-to-evict): true
        spec:          
          volumes: 
          - (emptyDir): {}
  - name: "annotate-host-path"
    match: 
      resources: 
        kinds: 
        - "Pod"
    mutate: 
      overlay:
        metadata:
          annotations:
            +(cluster-autoscaler.kubernetes.io/safe-to-evict): true
        spec:          
          volumes: 
          - (hostPath):
              path: "*"

````

