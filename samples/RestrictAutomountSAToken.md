# Restrict auto-mount of Service Account tokens

Kubernetes automatically mounts service account credentials in each pod. The service account may be assigned roles allowing pods to access API resources. To restrict access, opt out of auto-mounting tokens by setting `automountServiceAccountToken` to `false`.

## Policy YAML 

[restrict_automount_sa_token.yaml](more/restrict_automount_sa_token.yaml) 

````yaml
apiVersion : kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: restrict-automount-sa-token
spec:
  rules:
  - name: validate-automountServiceAccountToken
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Auto-mounting of Service Account tokens is not allowed"
      pattern:
        spec:
          automountServiceAccountToken: false
````



