## Description

This test verifies the resource validation webhook is configured correctly when a Policy targets all `*` resources.

## Steps

1.  - Create a Policy targeting `*`
    - Assert Policy gets ready
1.  - Assert that the resource validation webhook is configured correctly and scope is set to "namespaced"
