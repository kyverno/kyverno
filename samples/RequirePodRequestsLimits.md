# Require pod resource requests and limits

Application workloads share cluster resources. Hence, it is important to manage resources assigned to each pod.  It is recommended that `resources.requests.cpu`, `resources.requests.memory` and `resources.limits.memory` are configured per pod. Other resources such as GPUs may also be specified as needed.

If a namespace level request or limit is specified, defaults will automatically be applied to each pod based on the `LimitRange` configuration.

## Policy YAML

[require_pod_requests_limits.yaml](best_practices/require_pod_requests_limits.yaml)

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-pod-requests-limits
spec:
  validationFailureAction: audit
  rules:
  - name: validate-resources
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
````
