## Description

This test verifies the resource validation webhook is configured correctly when a Policy target the `Secret` resource and ClusterPolicy target the `ConfigMap` resource.

## Steps

1.  - Create a Policy targeting `Secret`
    - Create a ClusterPolicy targeting `ConfigMap`
    - Assert polices get ready
1.  - Assert that the resource validation webhook is configured correctly
