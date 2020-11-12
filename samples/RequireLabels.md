# Require labels

Labels are a fundamental and important way to assign descriptive metadata to Kubernetes resources, especially Pods. Labels are especially important as the number of applications grow and are composed in different ways.

This sample policy requires that the label `app.kubernetes.io/name` be defined on all Pods. If you wish to require that all Pods have multiple labels defined (as opposed to [any labels from an approved list](RequireCertainLabels.md)), this policy can be altered by adding an additional rule block which checks for a second (or third, etc.) label name.

## More Information

* [Common labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/)

## Policy YAML

[require_labels.yaml](best_practices/require_labels.yaml)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: audit
  rules:
  - name: check-for-labels
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "The label `app.kubernetes.io/name` is required."
      pattern:
        metadata:
          labels:
            app.kubernetes.io/name: "?*"
```
