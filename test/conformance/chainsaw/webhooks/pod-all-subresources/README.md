## Description

This test verifies the resource validation webhook is configured correctly when a policy targets all `Pod/*` subresources.

## Steps

1.  - Create a policy targeting `Pod/*`
    - Assert policy gets ready
1.  - Assert that the resource validation webhook is configured correctly
