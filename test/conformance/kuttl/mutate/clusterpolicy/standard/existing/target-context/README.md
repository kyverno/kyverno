## Description

This test creates one `ConfigMap` named `target`. 

It then creates a `ClusterPolicy` with a mutate existing rule targeting the previously created `ConfigMap`.

The policy rule uses `context` on the trigger resource to create a variable containing the value of `data.content`.
The policy rule uses `context` on the target resource to create a variable containing the value of `data.content`.
The policy mutates target resource, setting `data.content` to the value of the trigger resource level variable and `data.targetContent` to the value of the target resource level variable.

Finally, the test creates the trigger config map.

## Expected Behavior

The target config map should contain:

```yaml
data:
  content: trigger
  targetContent: target
```