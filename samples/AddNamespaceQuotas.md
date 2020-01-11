# Configure namespace limits and quotas

To limit the number of resources like CPU and memory, as well as objects that may be consumed by workloads in a namespace, it is important to configure resource limits and quotas for each namespace. The generated default limitrange sets the default quotas for a container.

## Additional Information

* [Resource Quota](https://kubernetes.io/docs/concepts/policy/resource-quotas/)

## Policy YAML 

[add_ns_quota.yaml](best_practices/add_ns_quota.yaml) 

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: add-ns-quota
spec:
  rules:
  - name: generate-resourcequota
    match:
      resources:
        kinds:
        - Namespace
    generate:
      kind: ResourceQuota
      name: default-resourcequota
      data:
        spec:
          hard:
            requests.cpu: '4'
            requests.memory: '16Gi'
            limits.cpu: '4'
            limits.memory: '16Gi'
  - name: generate-limitrange
    match:
      resources:
        kinds:
        - Namespace
    generate:
      kind: LimitRange
      name: default-limitrange
      data:
        spec:
          limits:
          - default:
              cpu: 500m
              memory: 1Gi
            defaultRequest:
              cpu: 200m
              memory: 256Mi
            type: Container
````