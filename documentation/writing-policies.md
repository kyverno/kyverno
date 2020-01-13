<small>*[documentation](/README.md#documentation) / Writing Policies*</small>

# Writing Policies

The following picture shows the structure of a Kyverno Policy:

![KyvernoPolicy](images/Kyverno-Policy-Structure.png)

Each Kyverno policy contains one or more rules. Each rule has a match clause, an optional excludes clause, and a mutate, validate, or generate clause.

When Kyverno receives an admission controller request, i.e. a validation or mutation webhook, it first checks to see if the resource and user information matches or should be excluded from processing. If both checks pass, then the rule logic to mutate, validate, or generate resources is applied.

The following YAML provides an example for the match and validate clauses.

````yaml
apiVersion : kyverno.io/v1
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
          namespaces: # Optional, list of namespaces. Supports wilcards * and ?
          - "dev*"
          - test
          selector: # Optional, a resource selector is optional. Selector values support wildcards * and ?
              matchLabels:
                  app: mongodb
              matchExpressions:
                  - {key: tier, operator: In, values: [database]}
        # Optional, subjects to be matched
        subjects:
        - kind: User
          name: mary@somecorp.com
        # Optional, roles to be matched
        roles:
        # Optional, clusterroles to be matched
        clusterroles:
      # Resources that need to be excluded
      exclude: # Optional, resources to be excluded from evaulation
        resources:
          kinds:
          - Daemonsets
          name: "*"
          namespaces:
          - prod
          - "kube*"
          selector:
              matchLabels:
                  app: mongodb
              matchExpressions:
                  - {key: tier, operator: In, values: [database]}
        # Optional, subjects to be excluded
        subjects:
        # Optional, roles to be excluded
        roles:
        # Optional, clusterroles to be excluded
        clusterroles:
          - cluster-admin
          - admin
      # rule is evaluated if the preconditions are satisfied
      # all preconditions are AND/&& operation
      preconditions:
      - key: name # compares (key operator value) 
        operator: Equal
        value: name # constant "name" == "name"
      - key: "{{serviceAccountName}}" # refer to a pre-defined variable serviceAccountName
        operator: NotEqual
        value: "user1" # if service 
     # Each rule can contain a single validate, mutate, or generate directive
     ...
````

Each rule can validate, mutate, or generate configurations of matching resources. A rule definition can contain only a single **mutate**, **validate**, or **generate** child node. These actions are applied to the resource in described order: mutation, validation and then generation.

# Variables:
Variables can be used to reference attributes that are loaded in the context using a [JMESPATH](http://jmespath.org/) search path.
Format: `{{<JMESPATH>}}`
Resources available in context:
- Resource: `{{request.object}}`
- UserInfo: `{{request.userInfo}}`

## Pre-defined Variables
- `serviceAccountName` : the variable removes the suffix system:serviceaccount:<namespace>: and stores the userName. 
Example  userName=`system:serviceaccount:nirmata:user1` will store variable value as `user1`.
- `serviceAccountNamespace` : extracts the `namespace` of the serviceAccount. 
Example  userName=`system:serviceaccount:nirmata:user1` will store variable value as `nirmata`.

Examples:

1. Refer to resource name(type string)

`{{request.object.metadata.name}}`

2. Build name from multiple variables(type string)

`"ns-owner-{{request.object.metadata.namespace}}-{{request.userInfo.username}}-binding"`

3. Refer to metadata struct/object(type object)

`{{request.object.metadata}}`

# PreConditions:
Apart from using `match` & `exclude` conditions on resource to filter which resources to apply the rule on, `preconditions` can be used to define custom filters.
```yaml
  - name: generate-owner-role
    match:
      resources:
        kinds:
        - Namespace
    preconditions:
    - key: "{{request.userInfo.username}}"
      operator: NotEqual
      value: ""
```
In the above example, if the variable `{{request.userInfo.username}}` is blank then we dont apply the rule on resource.

Operators supported:
- Equal
- NotEqual

---
<small>*Read Next >> [Validate](/documentation/writing-policies-validate.md)*</small>