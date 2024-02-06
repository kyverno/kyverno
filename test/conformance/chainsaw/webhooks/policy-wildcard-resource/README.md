## Description

This test verifies the resource validation webhook is configured correctly when a ClusterPolicy targets all `*` resources.

## Steps

1.  - Create a ClusterPolicy targeting `*`
    - Assert policy gets ready
1.  - Assert that the resource validation webhook is configured correctly
