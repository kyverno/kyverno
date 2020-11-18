# Disallow Secrets from environment variables

Secrets in Kubernetes are often sensitive pieces of information whose content should be protected. Although they can be used in many ways, when mounting them as environment variables, some applications can write their values to STDOUT revealing this sensitive information in log files and potentially other exposure. As a best practice, Kubernetes Secrets should be mounted instead as volumes.

This sample policy checks any incoming Pod manifests and ensures that Secrets are not mounted as environment variables.

## More Information

* [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)

## Policy YAML

[disallow_secrets_from_env_vars.yaml](more/disallow_secrets_from_env_vars.yaml)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: secrets-not-from-env-vars
spec:
  background: false
  validationFailureAction: audit
  rules:
  - name: secrets-not-from-env-vars
    match:
    resources:
        kinds:
        - Pod
    validate:
      message: "Secrets must be mounted as volumes, not as environment variables."
      pattern:
        spec:
          containers:
          - name: "*"
            =(env):
            - =(valueFrom):
                X(secretKeyRef): "null"
```
