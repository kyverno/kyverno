## Description

The policy should not contain autogen rules as autogen should not apply to the policy (it's not a `Pod` only policy).

## Expected Behavior

The policy gets created and contains no autogen rules in the status.
