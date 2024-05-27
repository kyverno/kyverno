# Kverno Validation Rule Grammar

This grammar describes the structure of Kverno validate rules. It uses a simplified notation similar to Extended Backus-Naur Form (EBNF).

```ebnf
Rule = "name:" RuleName
       "match:" Match
       ["exclude:" Exclude]
       "validate:" Validate

RuleName = string

Match = "any:" MatchBlock | "all:" MatchBlock

MatchBlock = { MatchElement }

MatchElement = "resources:" ResourceFilter
               | "subjects:" SubjectFilter
               | "roles:" RoleFilter
               | "clusterRoles:" ClusterRoleFilter

ResourceFilter = { "kinds:" [ "*" | { ResourceKind } ]
                    | "names:" [ "*" | { string } ]
                    | "namespaces:" [ "*" | { string } ]
                    | "operations:" [ { Operation } ]
                    | "selector:" LabelSelector
                    | "namespaceSelector:" LabelSelector
                    | "annotations:" { string: string } }

ResourceKind = string [ "/" string [ "/" string ] ] | string [ "." string [ "." string ] ]

Operation = "CREATE" | "UPDATE" | "DELETE" | "CONNECT"

SubjectFilter = { "kind:" SubjectKind
                  | "name:" string }

SubjectKind = "User" | "Group" | "ServiceAccount"

RoleFilter = { string }

ClusterRoleFilter = { string }

LabelSelector = { "matchLabels:" { string: string }
                   | "matchExpressions:" { LabelSelectorRequirement } }

LabelSelectorRequirement = { "key:" string
                              | "operator:" LabelSelectorOperator
                              | "values:" { string } }

LabelSelectorOperator = "In" | "NotIn" | "Exists" | "DoesNotExist"

Exclude = Match

Validate = "message:" string
           ( "pattern:" Pattern
             | "anyPattern:" { Pattern }
             | "deny:" Deny )
           [ "foreach:" { Foreach } ]

Pattern = JSON-like object

Deny = "conditions:" ( "any:" ConditionBlock | "all:" ConditionBlock )

ConditionBlock = { Condition }

Condition = "key:" string
            | "operator:" Operator
            | "value:" ( string | number | boolean | { string } | { number } )
            [ "message:" string ]

Operator = "Equals" | "NotEquals" | "AnyIn" | "AllIn" | "AnyNotIn" | "AllNotIn" 
          | "GreaterThan" | "GreaterThanOrEquals" | "LessThan" | "LessThanOrEquals" 
          | "DurationGreaterThan" | "DurationGreaterThanOrEquals" | "DurationLessThan" | "DurationLessThanOrEquals"

Foreach = "list:" JMESPathExpression
          ( "pattern:" Pattern
            | "anyPattern:" { Pattern }
            | "deny:" Deny )
          [ "context:" Context ]
          [ "preconditions:" Preconditions ]
          [ "elementScope:" boolean ]

JMESPathExpression = string

Context = { ContextEntry }

ContextEntry = "name:" string
               ( "configMap:" ConfigMapReference
                 | "apiCall:" APICall
                 | "variable:" Variable
                 | "imageRegistry:" ImageRegistryReference
                 | "globalReference:" GlobalReference )

ConfigMapReference = { "name:" string
                      | "namespace:" string }

APICall = { "urlPath:" string
            | "method:" ( "GET" | "POST" )
            | "data:" { "key:" string | "value:" string }
            | "service:" ServiceReference }

ServiceReference = { "url:" string
                     | "caBundle:" string }

Variable = { "value:" ( string | number | boolean | JSON-like object )
             [ "jmesPath:" JMESPathExpression ]
             [ "default:" ( string | number | boolean | JSON-like object ) ] }

ImageRegistryReference = { "reference:" string
                           [ "jmesPath:" JMESPathExpression ] }

GlobalReference = { "name:" string
                    [ "jmesPath:" JMESPathExpression ] }

Preconditions = "any:" ConditionBlock | "all:" ConditionBlock

```

**Notes:**

* `JSON-like object` refers to any valid JSON object structure.
* `string` refers to a string literal.
* `number` refers to a numeric literal.
* `boolean` refers to a boolean literal (`true` or `false`).
* Curly braces `{}` enclose a set of elements that can occur zero or more times.
* Square brackets `[]` enclose an optional element.
* Vertical bar `|` separates alternative choices.

## Construct-Based Definition

A Kverno Validation Rule is a construct that defines how to validate Kubernetes resources. It consists of the following components:

**1. Rule Name (`name`):** 
    - A unique identifier for the rule within the policy. 

**2. Resource Matching (`match`, `exclude`):**
    - Determines which resources the rule applies to.
        - `match`: Defines criteria for resources to be included.
        - `exclude`: Defines criteria for resources to be excluded.
    - Both `match` and `exclude` can have one of two structures:
        - `any`: Logical OR of multiple criteria.
        - `all`: Logical AND of multiple criteria.
    - Criteria can be based on:
        - Resource attributes: Kinds, names, namespaces, operations, labels, annotations, and namespace selectors.
        - Subjects: Users, groups, and service accounts.
        - Roles: Namespaced roles.
        - ClusterRoles: Cluster-wide roles.

**3. Validation Logic (`validate`):**
    - Defines how to validate the matched resources. 
    - Contains a message to be displayed on failure and one of the following validation types:
        - **Pattern Matching (`pattern`):**
            - Defines a desired configuration pattern. 
            - Resources must match this pattern to pass validation.
            - Supports wildcards (`*`, `?`) and operators (`>`, `<`, `>=`, `<=`, `!`).
            - Includes anchors for conditional processing (`()`, `=()`, `^()`, `X()`).

        - **Any Pattern Matching (`anyPattern`):**
            - Defines a list of alternative patterns.
            - Resources must match at least one of these patterns to pass validation.

        - **Deny Rules (`deny`):**
            - Defines conditions using expressions to deny requests.
            - Resources are denied if any/all of the conditions are TRUE.
            - Supports various operators for comparisons (`Equals`, `NotEquals`, `AnyIn`, `AllIn`, `AnyNotIn`, `AllNotIn`, `GreaterThan`, `LessThan`, etc.)
            - Provides greater flexibility for advanced validation scenarios.

**4. Iteration (`foreach`):**
    - Optional. Allows applying validation logic to elements within lists.
    - Defines a `list` of elements to iterate over using JMESPath expression.
    - Can be nested for iterating over multiple lists.
    - Supports `context`, `preconditions`, and `elementScope` within each loop.

**5. Context (`context`):**
    - Optional. Defines variables that can be used within the rule.
    - Variables can be defined from various sources:
        - Static values.
        - JMESPath expressions.
        - ConfigMaps.
        - Kubernetes API calls.
        - Service calls.
        - Image registries.
        - Global context entries.

**6. Preconditions (`preconditions`):**
    - Optional. Defines conditions using expressions to control when a rule is executed.
    - Uses the same structure as Deny Rules with `any` and `all` blocks.
    - Allows for fine-grained selection of resources based on variables and expressions.

**7. Common Expression Language (`cel`):**
    - Optional. Allows writing CEL expressions for resource validation.
    - The `cel.expressions` field contains CEL expressions to evaluate against the resource.
    - Supports access to various CEL variables including:
        - `object`, `oldObject`, `request`, `params`, `namespaceObject`, `authorizer`.

**8. Pod Security (`podSecurity`):**
    - Optional. Simplifies applying Pod Security Standards profiles and controls.
    - The `level` field defines the profile (baseline, restricted, privileged).
    - The `version` field specifies the Pod Security Standards version.
    - The `exclude` field allows exempting specific controls or images from the profile.

**9. Manifest Validation (`manifests`):**
    - Optional. Allows validating signed Kubernetes YAML manifests.
    - Requires the public key used for signing the manifest.
    - Supports ignoring specific fields for expected modifications.
    - Enables secure verification of manifests.

**10. Validating Admission Policies (`validatingadmissionpolicy`):**
    - Optional. Generates Kubernetes ValidatingAdmissionPolicies from Kyverno policies.
    - Enables in-process validation using CEL expressions directly within the API server.
    - Simplifies policy management by unifying validation logic.

## Basic Structure:

```yaml
# A Kverno policy contains one or more rules.
apiVersion: kyverno.io/v1
kind: ClusterPolicy # or Policy
metadata:
  name: policy-name # unique name for the policy

spec:
  # Policy-level settings (e.g., validationFailureAction, background, etc.)
  # ...

  rules: # List of rules within the policy
    - name: rule-name # Unique name for the rule
      match: # Resource matching criteria
        any: # Logical OR of criteria within this block
          - resources: # Match based on resource attributes
              kinds: # List of resource kinds to match
                - Pod # Example: Match Pods
                - Deployment # Example: Match Deployments
              # Other resource matching criteria (names, namespaces, operations, labels, annotations)
              # ...
        # Can have multiple 'any' or 'all' blocks for combining criteria
        # ...

      exclude: # Resource exclusion criteria (optional)
        # Similar structure as 'match'
        # ...

      validate: # Validation logic
        message: "Error message to display on validation failure." # Required
        # Choose one of the following validation methods:

        pattern: # Pattern matching (compare resource to a desired pattern)
          # JSON-like object representing the desired configuration pattern
          # ...

        anyPattern: # Any pattern matching (match at least one of multiple patterns)
          - # List of alternative patterns
            # ...

        deny: # Deny rules (define conditions for denying requests)
          conditions: # Define conditions using expressions
            any: # Logical OR of conditions within this block
              - key: "{{ expression }}" # JMESPath expression representing the key to check
                operator: "Equals" # Comparison operator (Equals, NotEquals, AnyIn, AllIn, etc.)
                value: "value-to-compare" # Value to compare against
                message: "Custom message to display if this condition fails" # Optional

        foreach: # Iterate over elements within lists (optional)
          - list: "JMESPath expression" # JMESPath expression defining the list to iterate over
            # Validation logic to apply to each element in the list (pattern, anyPattern, or deny)
            # ...

      context: # Define variables for use within the rule (optional)
        - name: variable-name # Name of the variable
          configMap: # Fetch data from a ConfigMap
            name: configmap-name
            namespace: configmap-namespace
          # Other context variable sources (apiCall, variable, imageRegistry, globalReference)
          # ...

      preconditions: # Define conditions to control rule execution (optional)
        # Similar structure as 'deny' conditions
        # ...

      cel: # Common Expression Language for validation (optional)
        expressions: # List of CEL expressions
          - expression: "CEL expression" # CEL expression to evaluate
            message: "Error message if the expression evaluates to false" # Required

      podSecurity: # Apply Pod Security Standards (optional)
        level: "baseline" # or "restricted" or "privileged"
        version: "latest" # or a specific version (e.g., "v1.24")
        exclude: # List of controls or images to exclude from the profile
          - controlName: "control-name"
            images: # Optional, used when excluding controls at the container level
              - "image-name"

      manifests: # Validate signed Kubernetes manifests (optional)
        attestors: # Define attestors for signature verification
          - count: 1 # Number of required signatures
            entries: # List of signature entries
              - keys: # Public keys for signature verification
                publicKeys: |- # Multiline string containing the public key(s)
                  #  -----BEGIN PUBLIC KEY-----
                  #  ...
                  #  -----END PUBLIC KEY-----
        ignoreFields: # List of fields to ignore during verification
          - objects:
              - kind: "resource-kind" # Resource kind to apply the ignore rule to
            fields: # List of fields to ignore
              - "field-path" # JMESPath expression for the field to ignore
```
