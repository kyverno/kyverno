## Description

This test tries to create policies referencing `target` in the trigger preconditions or context of a mutate existing rule.

## Expected Behavior

Policies shoudl be rejected.
Referencing `target` is only allowed in the target section of a mutate existing rule.