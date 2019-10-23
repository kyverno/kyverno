# Require pod resource requests and limits

As application workloads share cluster resources, it is important to limit resources requested and consumed by each pod. It is recommended to require `resources.requests` and `resources.limits` per pod. If a namespace level request or limit is specified, defaults will automatically be applied to each pod based on the `LimitRange` configuration. 

## Policy YAML 

[require_pod_requests_limits.yaml](best_practices/require_pod_requests_limits.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: check-resource
spec:
  validationFailureAction: "audit"
  rules:
  - name: check-resource-request-limit
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "CPU and memory resource requests and limits are required"
      pattern:
        spec:
          containers:
          - resources:
              requests:
                memory: "?*"
                cpu: "?*"
              limits:
                memory: "?*"
                cpu: "?*"
````
