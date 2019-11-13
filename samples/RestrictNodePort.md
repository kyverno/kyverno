# Restrict use of `NodePort` services

A Kubernetes service of type `NodePort` uses a host port (on every node in the cluster) to receive traffic from any source. 

Kubernetes Network Policies cannot be used to control traffic to host ports. 

Although NodePort services can be useful, their use should be limited to services with additional upstream security checks.

## Policy YAML

[restrict_node_port.yaml](more/restrict_node_port.yaml)

````yaml

apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: restrict-node-port
spec:
  rules:
  - name: validate-node-port
    match:
      resources:
        kinds:
        - Service
    validate:
      message: "Service of type NodePort is not allowed"
      pattern: 
        spec:
          type: "!NodePort"

````

