<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Validate*</small>


# Validate Configurations 

A validation rule is expressed as an overlay pattern that expresses the desired configuration. Resource configurations must match fields and expressions defined in the pattern to pass the validation rule. The following rules are followed when processing the overlay pattern:

1. Validation will fail if a field is defined in the pattern and if the field does not exist in the configuration. 
2. Undefined fields are treated as wildcards. 
3. A validation pattern field with the wildcard value '*' will match zero or more alphanumeric characters. Empty values or missing fields are matched.
4. A validation pattern field with the wildcard value '?' will match any single alphanumeric character. Empty or missing fields are not matched. 
5. A validation pattern field with the wildcard value '*?' will match any alphanumeric characters and requires the field to be present with non-empty values.
6. A validation pattern field with the value `null` requires that the field not be defined or have a null value.
6. The validation of siblings is performed only when one of the field values matches the value defined in the pattern. You can use the parenthesis operator to explictly specify a field value that must be matched. This allows writing rules like 'if fieldA equals X, then fieldB must equal Y'.
7. Validation of child values is only performed if the parent matches the pattern.

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
| `&`        | logical and               |

There is no operator for `equals` as providing a field value in the pattern requires equality to the value.

## Example

````yaml

apiVersion : kyverno.io/v1alpha1
kind : Policy
metadata :
  name : validation-example
spec :
  rules:
    - resource:
        # Kind specifies one or more resource types to match
        kind: Deployment, StatefuleSet, DaemonSet
        # Name is optional and can use wildcards
        name: *
        # Selector is optional
        selector:
      validate:
        # Message is optional
        message: "The label app is required"
        pattern:
          spec:
            selector:
              matchLabels:
                app: ?*

````

Additional examples are available in [examples](/examples/)


---
<small>*Read Next >> [Mutate](/documentation/writing-policies-mutate.md)*</small>
