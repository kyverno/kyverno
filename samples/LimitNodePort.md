# Limit `NodePort` services

A Kubernetes service of type `NodePort` uses a host port (on every node in the cluster) to receive traffic from any source. 

Kubernetes Network Policies cannot be used to control traffic to host ports. 

Although NodePort services can be useful, their use should be limited to services with additional upstream security checks.

## Policy YAML

[disallow_node_port.yaml](best_practices/disallow_node_port.yaml)

````yaml

apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: limit-node-port
spec:
  rules:
  - name: validate-node-port
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

