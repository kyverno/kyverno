## Description

This Chainsaw test attempts to create policies using `validate`, `mutate`, or `generate` rules that  include the `celPreconditions` field.

## Expected Behavior

The policy should be rejected, as `celPreconditions` are only allowed with the `validate.cel`.

## Related Issue

[kyverno/kyverno#13381](https://github.com/kyverno/kyverno/issues/13381)

