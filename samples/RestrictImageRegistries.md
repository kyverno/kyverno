# Disallow unknown image registries

Images from unknown registries may not be scanned and secured. Requiring the use of trusted registries helps reduce threat exposure and is considered a common Kubernetes best practice.

This sample policy requires that all images come from either `k8s.gcr.io` or `gcr.io`. You can customize this policy to allow other or different image registries that you trust. Alternatively, you can invert the check to allow images from all other registries except one (or a list) by changing the `image` field to `image: "!k8s.gcr.io"`.

## Policy YAML

[restrict_image_registries.yaml](more/restrict_image_registries.yaml)

````yaml
apiVersion : kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: restrict-image-registries
spec:
  validationFailureAction: audit
  rules:
  - name: validate-registries
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Unknown image registry."
      pattern:
        spec:
          containers:
          # Allows images from either k8s.gcr.io or gcr.io.
          - image: "k8s.gcr.io/* | gcr.io/*"
````
