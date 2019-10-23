# Disallow latest image tag

The `:latest` tag is mutable and can lead to unexpected errors if the upstream image changes. A best practice is to use an immutable tag that maps to a specific and tested version of an application pod.

## Policy YAML 

[require_image_tag_not_latest.yaml](best_practices/require_image_tag_not_latest.yaml)


````yaml
apiVersion : kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-image-tag
spec:
  rules:
  - name: image-tag-notspecified
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Image tag not specified"  
      pattern:
        spec:
          containers:
          - image: "*:*"
  - name: image-tag-not-latest
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Using 'latest' image tag is restricted. Set image tag to a specific version"
      pattern:
        spec:
          containers:
          - image: "!*:latest"
````
