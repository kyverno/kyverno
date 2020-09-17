---
title: Variables
slug: /variables
description: 
---

# Variables

Sometimes it is necessary to vary the contents of a mutated or generated resource based on request data. To achieve this, variables can be used to reference attributes that are loaded in the rule processing context using a [JMESPATH](http://jmespath.org/) notation. 

The policy engine will substitute any values with the format `{{ <JMESPATH> }}` with the variable value before processing the rule.

The following data is available for use in context:
- Resource: `{{request.object}}`
- UserInfo: `{{request.userInfo}}`

## Pre-defined Variables

Kyverno automatically creates a few useful variables:

- `serviceAccountName` : the "userName" which is last part of a service account i.e. without the prefix `system:serviceaccount:<namespace>:`. For example, when processing a request from `system:serviceaccount:nirmata:user1` Kyverno will store the value `user1` in the variable `serviceAccountName`.

- `serviceAccountNamespace` : the "namespace" part of the serviceAccount. For example, when processing a request from `system:serviceaccount:nirmata:user1` Kyverno will store `nirmata` in the variable `serviceAccountNamespace`.

## Examples

1. Reference a resource name (type string)

`{{request.object.metadata.name}}`

2. Build name from multiple variables (type string)

`"ns-owner-{{request.object.metadata.namespace}}-{{request.userInfo.username}}-binding"`

3. Reference the metadata (type object)

`{{request.object.metadata}}`
