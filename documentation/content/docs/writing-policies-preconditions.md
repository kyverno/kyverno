---
title: Preconditions
description: 
---

# Preconditions

Preconditions allow controlling policy rule execution based on variable values.

While `match` & `exclude` conditions allow filtering requests based on resource and user information, `preconditions` can be used to define custom filters for more granular control.

The following operators are currently supported for preconditon evaluation:
- Equal
- NotEqual
- In
- NotIn

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

```yaml
  - name: generate-default-build-role
    match:
      resources:
        kinds:
        - Namespace
    preconditions:
    - key: "{{serviceAccountName}}"
      operator: In
      value: ["build-default", "build-base"]
```

In the above example, the rule is only applied to requests from service account with name `build-default` and `build-base`.

