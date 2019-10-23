# Configure namespace limits and quotas

To limit the number of objects, as well as the total amount of compute that may be consumed by an application, it is important to create resource limits and quotas for each namespace.

## Policy YAML 

[require_namespace_quota.yaml](best_practices/require_namespace_quota.yaml) 

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-namespace-quota
spec:
  rules:
  - name: validate-namespace-quota
    match:
      resources:
        kinds:
        - Namespace
    generate:
      kind: ResourceQuota
      name: "defaultresourcequota"
      spec:
        hard:
          requests.cpu: "*"
          requests.memory: "*"
          limits.cpu: "*"
          limits.memory: "*"
````

## Additional Information

* [Resource Quota](https://kubernetes.io/docs/concepts/policy/resource-quotas/)

