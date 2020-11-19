# Add default labels to objects

Labels are important pieces of metadata that can be attached to just about anything in Kubernetes. They are often used to tag various resources as being associated in some way. Kubernetes has no ability to assign a series of "default" labels to incoming objects. This sample policy shows you how to assign one or multiple labels by default to any object you wish. Here it shows adding a label called `custom-foo-label` with value `my-bar-default` to resources of type `Pod`, `Service`, and `Namespace` but others can be added or removed as desired.

## Policy YAML

[add_default_labels.yaml](more/add_default_labels.yaml)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: add-default-labels
spec:
  background: false
  rules:
  - name: add-default-labels
    match:
      resources:
        kinds:
        - Pod
        - Service
        - Namespace
    mutate:
      patchStrategicMerge:
        metadata:
          labels:
            custom-foo-label: my-bar-default
```
