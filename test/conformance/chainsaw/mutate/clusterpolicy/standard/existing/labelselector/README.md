## Description

This test ensures that target resources for mutations can be selected using label selectors

## Expected Behavior

The target resource is fetched and mutated when specifying a label selector that will match it

## Steps

### Test Steps

1. Create three `ConfigMap` resources, two with the required label existing and one without it.
2. Create a `ClusterPolicy` that will add a label to `ConfigMaps` on any secret events, and select targets with the label.
3. Create a `Secert` resource.
4. Assert that the `ConfigMaps` got the required labels added to them.
5. Verify that the `ConfigMap` without the required label on it didn't get changed.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/10407