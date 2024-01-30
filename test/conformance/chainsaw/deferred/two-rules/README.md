## Description

This test checks that variables don't leak from one rule to the next.
The second rule tries to use a variable from the first rule, it should not find it.

## Expected Behavior

The configmap creates fine with the data:
```yaml
data:
    one: test
    two: "null"
```
