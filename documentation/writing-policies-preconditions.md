<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Preconditions*</small>

# Preconditions

Preconditions allow controlling policy rule execution based on variable values.

While `match` & `exclude` conditions allow filtering requests based on resource and user information, `preconditions` can be used to define custom filters for more granular control.

The following operators are currently supported for preconditon evaluation:
- Equal
- NotEqual

## Example

```yaml
  - name: generate-owner-role
    match:
      resources:
        kinds:
        - Namespace
    preconditions:
    - key: "{{serviceAccountName}}"
      operator: NotEqual
      value: ""
```

In the above example, the rule is only applied to requests from service accounts i.e. when the `{{serviceAccountName}}` is not empty.


<small>*Read Next >> [Auto-Generation for Pod Controllers](/documentation/writing-policies-autogen.md)*</small>
