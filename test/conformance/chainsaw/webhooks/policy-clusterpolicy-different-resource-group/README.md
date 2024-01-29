## Description

This test verifies the resource validation webhook is configured correctly when a ClusterPolicy targets CustomResourceDefinition resources and a Policy targets `ConfigMap`.

## Steps

1.  - Create a policy targeting `*`
    - Assert policy gets ready
1.  - Assert that the resource validation webhook is configured correctly two rules for `ConfigMap` and `CustomResourceDefinition` created
