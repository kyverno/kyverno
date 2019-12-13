<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Validate*</small>


# Validate Configurations 

A validation rule is expressed as an overlay pattern that expresses the desired configuration. Resource configurations must match fields and expressions defined in the pattern to pass the validation rule. The following rules are followed when processing the overlay pattern:

1. Validation will fail if a field is defined in the pattern and if the field does not exist in the configuration. 
2. Undefined fields are treated as wildcards. 
3. A validation pattern field with the wildcard value '*' will match zero or more alphanumeric characters. Empty values are matched. Missing fields are not matched.
4. A validation pattern field with the wildcard value '?' will match any single alphanumeric character. Empty or missing fields are not matched. 
5. A validation pattern field with the wildcard value '?*' will match any alphanumeric characters and requires the field to be present with non-empty values.
6. A validation pattern field with the value `null` or "" (empty string) requires that the field not be defined or has no value.
7. The validation of siblings is performed only when one of the field values matches the value defined in the pattern. You can use the parenthesis operator to explictly specify a field value that must be matched. This allows writing rules like 'if fieldA equals X, then fieldB must equal Y'.
8. Validation of child values is only performed if the parent matches the pattern.

## Patterns

### Wildcards
1. `*` - matches zero or more alphanumeric characters
2. `?` - matches a single alphanumeric character

### Operators

| Operator   | Meaning                   |
|------------|---------------------------| 
| `>`        | greater than              | 
| `<`        | less than                 | 
| `>=`       | greater than or equals to |
| `<=`       | less than or equals to    | 
| `!`        | not equals                |
|  \|        | logical or                |

There is no operator for `equals` as providing a field value in the pattern requires equality to the value.

## Anchors

Anchors allow conditional processing (i.e. "if-then-else) and other logical checks in validation patterns. The following types of anchors are supported:


| Anchor      	| Tag 	| Behavior                                                                                                                                                                                                                                     	|
|-------------	|-----	|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------	|
| Conditional 	| ()  	| If tag with the given value (including child elements) is specified, then peer elements will be processed. <br/>e.g. If image has tag latest then imagePullPolicy cannot be IfNotPresent. <br/>&nbsp;&nbsp;&nbsp;&nbsp;(image): "*:latest" <br>&nbsp;&nbsp;&nbsp;&nbsp;imagePullPolicy: "!IfNotPresent"<br/>                                             	|
| Equality    	| =() 	| If tag is specified, then processing continues. For tags with scalar values, the value must match. For tags with child elements, the child element is further evaluated as a validation pattern.  <br/>e.g. If hostPath is defined then the path cannot be /var/lib<br/>&nbsp;&nbsp;&nbsp;&nbsp;=(hostPath):<br/>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;path: "!/var/lib"<br/>                                                                                  	|
| Existence   	| ^() 	| Works on the list/array type only. If at least one element in the list satisfies the pattern. In contrast, a conditional anchor would validate that all elements in the list match the pattern. <br/>e.g. At least one container with image nginx:latest must exist. <br/>&nbsp;&nbsp;&nbsp;&nbsp;^(containers):<br/>&nbsp;&nbsp;&nbsp;&nbsp;- image: nginx:latest<br/>  	|
| Negation    	| X() 	| The tag cannot be specified. The value of the tag is not evaulated. <br/>e.g. Hostpath tag cannot be defined.<br/>&nbsp;&nbsp;&nbsp;&nbsp;X(hostPath):<br/>	|

## Anchors and child elements

Child elements are handled differently for conditional and equality anchors. 

For conditional anchors, the child element is considered to be part of the "if" clause, and all peer elements are considered to be part of the "then" clause. For example, consider the pattern:

````yaml
  pattern:
    metadata:
      labels:
        allow-docker: true
    spec:
      (volumes):
        (hostPath):
          path: "/var/run/docker.sock"
````

This reads as "If a hostPath volume exists and the path equals /var/run/docker.sock, then a label "allow-docker" must be specified with a value of true."

For equality anchors, a child element is considered to be part of the "then" clause. Consider this pattern:

````yaml
  pattern:
    spec:
      =(volumes):
        =(hostPath):
          path: "!/var/run/docker.sock"
````

This is read as "If a hostPath volume exists, then the path must not be equal to /var/run/docker.sock".


## Examples

The following rule prevents the creation of Deployment, StatefuleSet and DaemonSet resources without label 'app' in selector:

````yaml

apiVersion : kyverno.io/v1
kind : ClusterPolicy
metadata :
  name : validation-example
spec :
  rules:
    - name: check-label
      match:
        resources:
          # Kind specifies one or more resource types to match
          kinds:
            - Deployment
            - StatefuleSet
            - DaemonSet
          # Name is optional and can use wildcards
          name: "*"
          # Selector is optional
          selector:
      validate:
        # Message is optional, used to report custom message if the rule condition fails
        message: "The label app is required"    
        pattern:
          spec:
            template:
              metadata:
                labels:
                  app: "?*"

````

### Existence anchor: at least one

A variation of an anchor, is to check that in a list of elements at least one element exists that matches the patterm. This is done by using the ^(...) notation for the field.

For example, this pattern will check that at least one container has memory requests and limits defined and that the request is less than the limit:

````yaml
apiVersion : kyverno.io/v1
kind : ClusterPolicy
metadata :
  name : validation-example2
spec :
  rules:
    - name: check-memory_requests_link_in_yaml_relative
      match:
        resources:
          # Kind specifies one or more resource types to match
          kinds:
            - Deployment
          # Name is optional and can use wildcards
          name: "*"
          # Selector is optional
          selector:
      validate:
        pattern:
          spec:
            ^(containers):
              resources:
                requests:
                  memory: "$(<=./../../limits/memory)"
                limits:
                  memory: "2048Mi"
````

### Logical OR across validation patterns

In some cases content can be defined at a different level. For example, a security context can be defined at the Pod or Container level. The validation rule should pass if either one of the conditions is met. 

The `anyPattern` tag can be used to check if any one of the patterns in the list match. 

<small>*Note: either one of `pattern` or `anyPattern` is allowed in a rule, they both can't be declared in the same rule.*</small>

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: check-container-security-context
spec:
  rules:
  - name: check-root-user
    exclude:
      resources:
        namespaces: 
        - kube-system
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Root user is not allowed. Set runAsNonRoot to true."
      anyPattern:
        - spec:
            securityContext:
              runAsNonRoot: true
        - spec:
            containers:
            - name: "*"
              securityContext:
                runAsNonRoot: true
````

Additional examples are available in [samples](/samples/README.md)

## Validation Failure Action

The `validationFailureAction` attribute controls processing behaviors when the resource is not compliant with the policy. If the value is set to `enforce` resource creation or updates are blocked when the resource does not comply, and when the value is set to `audit` a policy violation is reported but the resource creation or update is allowed.

---
<small>*Read Next >> [Generate](/documentation/writing-policies-mutate.md)*</small>
