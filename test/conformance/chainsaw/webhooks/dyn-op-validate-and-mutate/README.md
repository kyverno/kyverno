## Description

This test verifies that operations configured dynmically are correct in both validatingadmissionwebhooks and mutating webhooks

## Steps

1.  - Create a policy with validate block and mutate block
    - Assert policy gets ready
1.  - Assert that the resource validation and mutation webhook is configured correctly
