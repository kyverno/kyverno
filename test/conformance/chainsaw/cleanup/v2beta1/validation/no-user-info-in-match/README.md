## Description

This test creates a cleanup policy containing user infos in `match` statement.
The creation should fail as cleanup policies with user infos are not allowed.

## Steps

1.  - Try create a couple of cleanup policies, expecting the creation to fail because they contain user infos
