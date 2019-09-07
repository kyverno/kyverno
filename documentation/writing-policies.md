<small>*[documentation](/README.md#documentation) / Writing Policies*</small>

# Writing Policies

A Kyverno policy contains a set of rules. Each rule matches resources by kind, name, or selectors.

````yaml
apiVersion : kyverno.io/v1alpha1
kind : ClusterPolicy
metadata :
  name : policy
spec :
  # 'enforce' to block resource request if any rules fail
  # 'audit' to allow resource request on failure of rules, but create policy violations to report them
  validationFailureAction: enforce
  # Each policy has a list of rules applied in declaration order
  rules:
    # Rules must have a unique name
    - name: "check-pod-controller-labels"      
      # Each rule matches specific resource described by "match" field.
      match:
        resources:
          kinds: # Required, list of kinds
          - Deployment
          - StatefulSet
          name: "mongo*" # Optional, a resource name is optional. Name supports wildcards * and ?
          namespaces: # Optional, list of namespaces
          - devtest2
          - devtest1
          selector: # Optional, a resource selector is optional. Selector values support wildcards * and ?
              matchLabels:
                  app: mongodb
              matchExpressions:
                  - {key: tier, operator: In, values: [database]}
      # Resources that need to be excluded
      exclude: # Optional, resources to be excluded from evaulation
        resources:
          kinds:
          - Daemonsets
          name: "*"
          namespaces:
          - devtest2
          selector:
              matchLabels:
                  app: mongodb
              matchExpressions:
                  - {key: tier, operator: In, values: [database]}
    
     # Each rule can contain a single validate, mutate, or generate directive
     ...
````

Each rule can validate, mutate, or generate configurations of matching resources. A rule definition can contain only a single **mutate**, **validate**, or **generate** child node. These actions are applied to the resource in described order: mutation, validation and then generation.

**Resource description:**
* ```match``` is a required key that defines the parameters which identify the resources that need to matched

* ```exclude``` is an option key to exclude resources from the application of the rule

---
<small>*Read Next >> [Validate](/documentation/writing-policies-validate.md)*</small>