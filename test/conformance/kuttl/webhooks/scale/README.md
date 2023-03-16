## Description

This test verifies the resource validation webhook is configured correctly when a policy targets all `Scale` resource.
It should be equivalent to using `*/scale`

## Steps

1.  - Create a policy targeting `Scale`
    - Assert policy gets ready
1.  - Assert that the resource validation webhook is configured correctly
