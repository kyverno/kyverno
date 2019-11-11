# Disallow Helm Tiller

Tiller has known security challenges. It requires adminstrative privileges and acts as a shared resource accessible to any authenticated user. Tiller can lead to privilge escalation as restricted users can impact other users.

## Policy YAML 

````yaml
apiVersion : kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: disallow-helm-tiller
spec:
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