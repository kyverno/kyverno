Apply Command - Valid Parameter Combinations and their interpretation!

| S.No | policy        | resource         | cluster   | namespace      | interpretation                                                                           |
| ---- |:-------------:| :---------------:| :--------:| :-------------:| :----------------------------------------------------------------------------------------| 
| 1.   | policy.yaml   | -r resource.yaml | false     |                | apply policy from file to the resource from file                                         |
| 2.   | policy.yaml   | -r resourceName  | true      |                | apply policy from file to the resource in cluster                                        |
| 3.   | policy.yaml   |                  | true      |                | apply policy from file to all the resources in cluster                                   |
| 4.   | policy.yaml   | -r resourceName  | true      | -n=namespace   | apply policy from file to the resource in cluster in mentioned namespace                 |
| 5.   | policy.yaml   |                  | true      | -n=namespace   | apply policy from file to all the resources in cluster in mentioned namespace            |

Use '--policy_report' with apply command to generate policy report.

Example:

Consider the following policy and resources:

policy.yaml
```apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-pod-requests-limits
  annotations:
    policies.kyverno.io/category: Workload Management
    policies.kyverno.io/description: As application workloads share cluster resources, it is important 
      to limit resources requested and consumed by each pod. It is recommended to require 
      'resources.requests' and 'resources.limits' per pod. If a namespace level request or limit is 
      specified, defaults will automatically be applied to each pod based on the 'LimitRange' configuration.
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
```

resource1.yaml
```apiVersion: v1
kind: Pod
metadata:
  name: nginx1
  labels:
    env: test
spec:
  containers:
  - name: nginx
    image: nginx
    imagePullPolicy: IfNotPresent
    resources:
      requests:
        memory: "64Mi"
        cpu: "250m"
      limits:
        memory: "128Mi"
        cpu: "500m"
```

resource2.yaml
```apiVersion: v1
kind: Pod
metadata:
  name: nginx2
  labels:
    env: test
spec:
  containers:
  - name: nginx
    image: nginx
    imagePullPolicy: IfNotPresent
```

Case 1: Apply policy manifest on resource manifest

```
kyverno apply policy.yaml -r resource1.yaml -r resource2.yaml --policy_report
```

Case 2: Apply policy manifest on cluster resource 

create above resource manifest in cluster.
```
kyverno apply policy.yaml -r nginx1 -r nginx2 --cluster --policy_report
```

Case 3: Apply policy manifest on all resource avaliable in cluster
```
kyverno apply policy.yaml --cluster --policy_report
```
This will validate all the pods avaliable in cluster.

Case 4: Apply policy manifest on resource avaliable in cluster under a specific namspace
```
kyverno apply policy.yaml -r nginx1 -r nginx2 --cluster --policy_report -n default
```

Case 5: Apply policy manifest on all resource avaliable in cluster under a specific namespace
```
kyverno apply policy.yaml --cluster --policy_report -n default
```
This will validate all the pods avaliable in cluster avaliable under default namespace.

On applying policy.yaml on the mentioned resources, the following report will be generated: 

```apiVersion: policy.k8s.io/v1alpha1
kind: ClusterPolicyReport
metadata:
  name: clusterpolicyreport
results:
- message: 'Validation error: CPU and memory resource requests and limits are required; Validation rule validate-resources failed at path /spec/containers/0/resources/requests/'
  policy: require-pod-requests-limits
  resources:
  - apiVersion: v1
    kind: Pod
    name: nginx2
    namespace: default
    uid: 5fc041f2-b479-4b65-94b8-33e955f6f0d3
  rule: validate-resources
  scored: true
  status: fail
summary:
  fail: 1
```
