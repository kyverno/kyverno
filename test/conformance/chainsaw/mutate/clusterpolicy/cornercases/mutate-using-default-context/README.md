## Description

This test checks that the `default` field in a context variable replaces nil results during admission mutation.

## Expected Behavior

The context variable `podName` defaults to `empty` for `bad-box`, so the precondition matches and the
admitted Pod receives the label `foo: bar`.

The Pod selects a nonexistent test node because this test only needs admission, not a running container.
This keeps cleanup independent of image pulls, container startup, and kubelet termination.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7148