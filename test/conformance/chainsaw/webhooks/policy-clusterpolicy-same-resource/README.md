## Description

This test verifies the resource validation webhook is configured correctly when a Policy and ClusterPolicy target the `ConfigMap` resource.

## Steps

1.  - Create a Policy targeting `ConfigMap`
    - Create a ClusterPolicy targeting `ConfigMap`
    - Assert polices get ready
1.  - Assert that the resource validation webhook is configured correctly
