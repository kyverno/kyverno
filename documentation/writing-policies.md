<small>*[documentation](/README.md#documentation) / Writing Policies*</small>

# Writing Policies

A Kyverno policy contains a set of rules. Each rule matches resources by kind, name, or selectors.

````yaml
apiVersion : kyverno.io/v1alpha1
kind : Policy
metadata :
  name : policy
spec :

  # Each policy has a list of rules applied in declaration order
  rules:

    # Rules must have a name
    - name: "check-pod-controller-labels"
      
      # Each rule matches specific resource described by "resource" field.
      resource:
        kind: Deployment, StatefulSet, DaemonSet
        # Name is optional. By default validation policy is applicable to any resource of supported kinds.
        # Name supports wildcards * and ?
        name: "*"
        # Selector is optional and can be used to match specific resources
        # Selector values support wildcards * and ?
        selector:
            # A selector can use match
            matchLabels:
                app: mongodb
            matchExpressions:
                - {key: tier, operator: In, values: [database]}


     # Each rule can contain a single validate, mutate, or generate directive
     ...
````

Each rule can validate, mutate, or generate configurations of matching resources. A rule definition can contain only a single **mutate**, **validate**, or **generate** child node. These actions are applied to the resource in described order: mutation, validation and then generation.

---
<small>*Read Next >> [Validate](/documentation/writing-policies-validate.md)*</small>