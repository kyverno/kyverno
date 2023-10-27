## Description

This test verifies the resource validation webhook is configured correctly when a policy targets all `*/scale` subresources.

## Steps

1.  - Create a policy targeting `*/scale`
    - Assert policy gets ready
1.  - Assert that the resource validation webhook is configured correctly
