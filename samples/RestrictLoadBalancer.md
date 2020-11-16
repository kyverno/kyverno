# Restrict use of `LoadBalancer` services

A Kubernetes service of type `LoadBalancer` typically requires the use of a cloud provider to realize the infrastructure on the backend. Doing so has the side effect of increased cost and potentially bypassing existing `Ingress` resource(s) which are preferred methods of issuing traffic to a Kubernetes cluster. The use of Services of type `LoadBalancer` should therefore be carefully controlled or restricted across the cluster.

This sample policy checks for any services of type `LoadBalancer`. Change `validationFailureAction` to `enforce` to block their creation.

## Policy YAML

[restrict_loadbalancer.yaml](more/restrict_loadbalancer.yaml)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: no-loadbalancers
spec:
  validationFailureAction: audit
  rules:
  - name: no-LoadBalancer
    match:
      resources:
        kinds:
        - Service
    validate:
      message: "Service of type LoadBalancer is not allowed."
      pattern:
        spec:
          type: "!LoadBalancer"
```
