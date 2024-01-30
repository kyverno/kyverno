## Description

This test verifies the resource validation webhook is configured correctly when a Policy targets the `Secret` resource and ClusterPolicy targets the `ConfigMap` resource.

## Steps

1.  - Create a Policy targeting `Secret`
    - Create a ClusterPolicy targeting `ConfigMap`
    - Assert polices get ready
1.  - Assert that the resource validation webhook is configured correctly
