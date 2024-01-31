## Description

This test verifies the resource validation webhook is configured correctly when a ClusterPolicy targets the `ConfigMap` and `CustomResourceDefinition` resource.

## Steps

1.  - Create a ClusterPolicy targeting `ConfigMap` and `CustomResourceDefinition`
    - Assert polices get ready
1.  - Assert that the resource validation webhook is configured correctly
