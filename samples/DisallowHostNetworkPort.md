
# Disallow `hostNetwork` and `hostPort`

Using `hostPort` and `hostNetwork` allows pods to share the host networking stack allowing potential snooping of network traffic across application pods. 

## Policy YAML

[disallow_host_network_port.yaml](best_practices/disallow_host_network_port.yaml)


````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: disallow-host-network-port
spec:
  rules:
  - name: validate-host-network-port
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Using host networking is not allowed"
      pattern:
        spec:
          (hostNetwork): false
          containers:
          - name: "*"
            ports:
            - hostPort: null
````