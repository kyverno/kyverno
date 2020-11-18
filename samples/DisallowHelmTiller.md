# Disallow Helm Tiller

Tiller, in the [now-deprecated Helm v2](https://helm.sh/blog/helm-v2-deprecation-timeline/), has known security challenges. It requires administrative privileges and acts as a shared resource accessible to any authenticated user. Tiller can lead to privilge escalation as restricted users can impact other users.

## Policy YAML

[disallow_helm_tiller.yaml](best_practices/disallow_helm_tiller.yaml)

````yaml
apiVersion : kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-helm-tiller
spec:
  validationFailureAction: audit
  rules:
  - name: validate-helm-tiller
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Helm Tiller is not allowed"  
      pattern:
        spec:
          containers:
          - name: "*"
            image: "!*tiller*"
````
