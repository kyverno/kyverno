# Default deny all ingress traffic

By default, Kubernetes allows communications across all pods within a cluster. Network policies and, a CNI that supports network policies, must be used to restrict communinications. 

A default `NetworkPolicy` should be configured for each namespace to default deny all ingress traffic to the pods in the namespace. Application teams can then configure additional `NetworkPolicy` resources to allow desired traffic to application pods from select sources.

## Policy YAML 

[add_network_policy.yaml](best_practices/add_network_policy.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: add-networkpolicy
spec:
  rules:
  - name: "default-deny-ingress"
    match:
      resources: 
        kinds:
        - Namespace
        name: "*"
    generate: 
      kind: NetworkPolicy
      name: default-deny-ingress
      data:
        spec:
          # select all pods in the namespace
          podSelector: {}
          policyTypes: 
          - Ingress
````