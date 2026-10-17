## Description

This test uses a nested foreach to remove all env variables from all containers. The nested foreach
relies on the numbered variables Kyverno creates at runtime (`element0.env`, `{{elementIndex0}}` and
`{{elementIndex1}}`), which must be accepted by variable validation.

## Expected Behavior

The policy is admitted and the created pod contains the same containers as the original pod with all
env variables in all containers removed.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/17283
