# Require `imagePullPolicy` is set to `Always` for images not using `latest` tags

By default, Kubernetes sets the `imagePullPolicy` for images which specify a tag to be `IfNotPresent`. In some cases, this may not be desired where the image could be rebuilt upstream. This sample policy ensures that all containers have their `imagePullPolicy` set to `Always`.

## Policy YAML

[imagepullpolicy-always.yaml](misc/imagepullpolicy-always.yaml)

```yaml
apiVersion : kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: imagepullpolicy-always
spec:
  validationFailureAction: audit
  background: false
  rules:
  - name: imagepullpolicy-always
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "The imagePullPolicy must be set to `Always` for all containers when a tag other than `latest` is used."  
      pattern:
        spec:
          containers:
          - imagePullPolicy: Always
```