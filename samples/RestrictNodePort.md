# Restrict service type `NodePort`  

A Kubernetes service of type NodePort uses a host port to receive traffic from any source. A `NetworkPolicy` resource cannot be used to control traffic to host ports. Although `NodePort` services can be useful, their use must be limited to services with additional upstream security checks. 

## Policy YAML 

[disallow_node_port.yaml](best_practices/disallow_node_port.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: disallow-node-port
spec:
  rules:
  - name: disallow-node-port
    match:
      resources:
        kinds:
        - Service
    validate:
      message: "Disallow service of type NodePort"
      pattern: 
        spec:
          type: "!NodePort"
````