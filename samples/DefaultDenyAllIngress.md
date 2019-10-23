# Default deny all ingress traffic

By default, Kubernetes allows all ingress and egress traffic to and from pods within a cluster. A "default" `NetworkPolicy` resource for a namespace should be used to deny all ingress traffic to the pods in that namespace. Additional `NetworkPolicy` resources can then be configured to allow desired traffic to application pods.

## Policy YAML 

[require_default_network_policy.yaml](best_practices/require_default_network_policy.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: default-deny-ingress-networkpolicy
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