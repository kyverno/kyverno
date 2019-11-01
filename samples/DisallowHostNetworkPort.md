
# Disallow `hostNetwork` and `hostPort`

Using `hostPort` and `hostNetwork` allows pods to share the host networking stack allowing potential snooping of network traffic across application pods. 

## Policy YAML

[disallow_host_network_hostport.yaml](best_practices/disallow_host_network_hostport.yaml)


````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-host-network-hostport
spec:
  rules:
  - name: validate-host-network-hostport
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Defining hostNetwork and hostPort are not allowed"
      pattern:
        spec:
          (hostNetwork): false
          containers:
          - name: "*"
            ports:
            - hostPort: null
````