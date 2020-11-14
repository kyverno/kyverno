# Require deployments have multiple replicas

Deployments with only a single replica produce availability concerns should that single replica fail. In most cases, you would want Deployment objects to have more than one replica to ensure continued availability if not scale.

This sample policy requires that Deployments have more than one replica excluding a list of system namespaces.

## More Information

* [Kubernetes Deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)

## Policy YAML

[require_deployments_have_multiple_replicas.yaml](more/require_deployments_have_multiple_replicas.yaml)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: deployment-has-multiple-replicas
spec:
  validationFailureAction: audit
  rules:
    - name: deployment-has-multiple-replicas
      match:
        resources:
          kinds:
            - Deployment
      exclude:
        resources:
          namespaces:
          - kyverno
          - kube-system
          - kube-node-lease
          - kube-public
      validate:
        message: "Deployments must have more than one replica to ensure availability."
        pattern:
          spec:
            replicas: ">1"
```
