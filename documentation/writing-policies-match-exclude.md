<small>*[documentation](/README.md#documentation) / Writing Policies / Match & Exclude *</small>

# Match & Exclude

The `match` and `exclude` filters control which resources policies are applied to. 

The match / exclude clauses have the same structure, and can each contain the following elements:
* resources: select resources by name, namespaces, kinds, and label selectors.
* subjects: select users, user groups, and service accounts
* roles: select namespaced roles
* clusterroles: select cluster wide roles

At least one element must be specified in a `match` block. The `kind` attribute is optional, but if it's not specified the policy rule will only be applicable to metatdata that is common across all resources kinds.

When Kyverno receives an admission controller request, i.e. a validation or mutation webhook, it first checks to see if the resource and user information matches or should be excluded from processing. If both checks pass, then the rule logic to mutate, validate, or generate resources is applied.

The following YAML provides an example for a match clause.

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: policy
spec:
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
          name: "mongo*" # Optional, a resource name is optional. Name supports wildcards (* and ?)
          namespaces: # Optional, list of namespaces. Supports wildcards (* and ?)
          - "dev*"
          - test
          selector: # Optional, a resource selector is optional. Values support wildcards (* and ?)
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
        clusterroles: cluster-admin

        ...

````

All`match` and `exclude` element must be satisfied for the resource to be selected as a candidate for the policy rule. In other words, the match and exclude conditions are evaluated using a logical AND operation.

Here is an example of a rule that matches all pods, excluding pods created by using the `cluster-admin` cluster role.

````yaml
spec:
  rules:
    name: "match-pods-except-admin"
    match:
      resources:
        kinds:
        - Pod
    exclude:
      clusterroles: cluster-admin
````

This rule that matches all pods, excluding pods in the `kube-system` namespace.

````yaml
spec:
  rules:
    name: "match-pods-except-admin"
    match:
      resources:
        kinds:
        - Pod
    exclude:
      namespaces:
      - "kube-system"
````

Condition checks inside the `resources` block follow the logic "**AND across types but an OR inside list types**". For example, if a rule match contains a list of kinds and a list of namespaces, the rule will be evaluated if the request contains any one (OR) of the kinds AND any one (OR) of the namespaces. Conditions inside `clusterRoles`, `roles` and `subjects` are always evaluated using a logical OR operation, as each request can only have a single instance of these values.

This is an example that select Deployment **OR** StatefulSet that has label `app=critical`.

````yaml
spec:
  rules:
    - name: match-critical-app
      match:
        resources:    # AND across types but an OR inside types that take a list
          kinds:
          - Deployment,StatefulSet
          selector:
            matchLabels:
              app: critical
````

The following example matches all resources with label `app=critical` excluding the resource created by clusterRole `cluster-admin` **OR** by the user `John`.

````yaml
spec:
  rules:
    - name: match-criticals-except-given-rbac
      match:
        resources:
          selector:
            matchLabels:
              app: critical
      exclude:
        clusterRoles:
        - cluster-admin
        subjects:
        - kind: User
          name: John
````

---
<small>*Read Next >> [Validate Resources](/documentation/writing-policies-validate.md)*</small>
