# Add default labels to objects

Labels are important pieces of metadata that can be attached to just about anything in Kubernetes. They are often used to tag various resources as being associated in some way. Kubernetes has no ability to assign a series of "default" labels to incoming objects. This sample policy shows you how to assign one or multiple labels by default to any object you wish. Here it shows adding a label called `custom-foo-label` with value `my-bar-default` to resources of type `Pod`, `Service`, and `Namespace` but others can be added or removed as desired.

Alternatively, you may wish to only add the `custom-foo-label` if it is not already present in the creation request. For example, if a user/process submits a request for a new `Namespace` object and the manifest already includes the label `custom-foo-label` with a value of `custom-value`, Kyverno can leave this label untouched which results in the newly-created object having the label `custom-foo-label=custom-value` instead of `my-bar-default`. In order to do this, enclose the label in the sample manifest in `+()` so the key name becomes `+(custom-foo-label)`. This conditional instructs Kyverno to only add the label if absent.

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
