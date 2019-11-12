# Disallow unknown image registries

Images from unknown registries may not be scanned and secured. Requiring the use of trusted registries helps reduce threat exposure. 

You can customize this policy to allow image registries that you trust.

## Policy YAML 

[trusted_image_registries.yaml](best_practices/trusted_image_registries.yaml) 

````yaml
apiVersion : kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: trusted-registries
spec:
  rules:
  - name: trusted-registries
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Unknown image registry"
      pattern:
        spec:
          containers:
          - image: "k8s.gcr.io/* | gcr.io/*"
````
