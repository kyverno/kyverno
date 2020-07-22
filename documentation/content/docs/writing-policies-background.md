---
title: Background processing 
description: 
---

# Background processing

Kyverno applies policies during admission control and to existing resources in the cluster that may have been created before a policy was created. The application of policies to existing resources is referred to as `background` processing. 

Note, that Kyverno does not mutate existing resources, and will only report policy violation for existing resources that do not match mutation, validation, or generation rules.

A policy is always enabled for processing during admission control. However, policy rules that rely on request information (e.g. `{{request.userInfo}}`) cannot be applied to existing resource in the `background` mode as the user information is not available outside of the admission controller. Hence, these rules must use the boolean flag `{spec.background}` to disable `background` processing.

```
spec:
  background: true
  rules:
  - name: default-deny-ingress
```

The default value of `background` is `true`. When a policy is created or modified, the policy validation logic will report an error if a rule uses `userInfo` and does not set `background` to `false`.
