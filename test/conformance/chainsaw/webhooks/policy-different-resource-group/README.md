## Description

This test verifies the resource validation webhook is configured correctly when a Policy targets all `*` resources.

## Steps

1.  - Create a policy targeting `Deployment`
      Create a policy targeting `Configmap`
    - Assert policies gets ready
1.  - Assert that the resource validation webhook is configured correctly and two rules are created with scope is set to "namespaced"
