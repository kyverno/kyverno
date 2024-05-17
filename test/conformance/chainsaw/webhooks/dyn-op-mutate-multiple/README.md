## Description

This test verifies that operations configured dynamically are correct in mutatingwebhookconfiguration with multiple policies
## Steps

1.  - Create 2 policies with mutate
    - Assert policy gets ready
2.  - Assert that the resource mutation webhook is configured correctly
