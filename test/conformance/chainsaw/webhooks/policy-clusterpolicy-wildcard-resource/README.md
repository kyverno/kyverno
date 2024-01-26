## Description

This test verifies the resource validation webhook is configured correctly when a Policy and ClusterPolicy target all `*` resources.

## Steps

1.  - Create a Policy targeting `*`
    - Create a ClusterPolicy targeting `*`
    - Assert policies get ready
1.  - Assert that the resource validation webhook is configured correctly
