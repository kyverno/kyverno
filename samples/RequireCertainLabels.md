# Require certain labels

In many cases, you may require that at least a certain number of labels are assigned to each Pod from a select list of approved labels. This sample policy demonstrates the [`anyPattern`](https://kyverno.io/docs/writing-policies/validate/#anypattern---logical-or-across-multiple-validation-patterns) option in a policy by requiring any of the two possible labels defined within. A pod must either have the label `app.kubernetes.io/name` or `app.kubernetes.io/component` defined. If you would rather validate that all Pods have multiple labels in an AND fashion rather than OR, check out the [require_labels](RequireLabels.md) example.

## Policy YAML

[require_certain_labels.yaml](best_practices/require_certain_labels.yaml)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-certain-labels
spec:
  validationFailureAction: audit
  rules:
  - name: validate-certain-labels
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "The label `app.kubernetes.io/name` or `app.kubernetes.io/component` is required."
      anyPattern:
      - metadata:
          labels:
            app.kubernetes.io/name: "?*"
      - metadata:
          labels:
            app.kubernetes.io/component: "?*"
```
