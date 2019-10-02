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
| Anchor      	| Tag 	| Behavior                                                                                                                                                                                                                                     	|
|-------------	|-----	|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------	|
| Conditional 	| ()  	| If tag with the given value is specified, then following resource elements must satisfy the conditions.<br/>e.g. <br/><code> (image):"*:latest" <br/>  imagePullPolicy: "!IfNotPresent"</code>  <br/> If image has tag latest then, imagePullPolicy cannot be IfNotPresent.                                                	|
| Equality    	| =() 	| if tag is specified, then it should have the provided value.<br/>e.g.<br/><code> =(hostPath):<br/> path: "!/var/lib" </code><br/> If hostPath is defined then the path cannot be /var/lib                                                                                  	|
| Existance   	| ^() 	| It can be specified on the list/array type only. If there exists at least one resource in the list that satisfies the pattern.<br/>e.g. <br/><code> ^(containers):<br/> - image: nginx:latest </code><br/> There must exist at least one container with image nginx:latest. 	|
## Example
The next rule prevents the creation of Deployment, StatefuleSet and DaemonSet resources without label 'app' in selector:
````yaml

apiVersion : kyverno.io/v1alpha1
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

### Check if one exist
A variation of an anchor, is to check existance of one element. This is done by using the ^(...) notation for the field.

For example, this pattern will check the existance of "name" field in the list:

````yaml
apiVersion : kyverno.io/v1alpha1
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
            - (name): "*"
              resources:
                requests:
                  memory: "$(<=./../../limits/memory)"
                limits:
                  memory: "2048Mi"
````

### Allow OR across overlay pattern
In some cases one content can be defined at a different level. For example, a security context can be defined at the Pod or Container level. The validation rule should pass if one of the conditions is met. 
`anyPattern` can be used to check on at least one of condition, it is the array of pattern, and the rule will be passed if at least one pattern is true.

<small>*Note: either `pattern` or `anyPattern` is allowed in each rule, they can't be decalred in the same rule.*</small>

````yaml
apiVersion: kyverno.io/v1alpha1
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


Additional examples are available in [examples](/examples/)


---
<small>*Read Next >> [Generate](/documentation/writing-policies-mutate.md)*</small>
