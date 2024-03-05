## Description

This test verifies the resource mutation webhook is configured correctly when a policy targets the `Pod/exec` subresource.

## Steps

1.  - Create a policy targeting `Pod/exec`
    - Assert policy gets ready
1.  - Assert that the resource mutation webhook is configured correctly

## Reference Issue(s)

#9829
