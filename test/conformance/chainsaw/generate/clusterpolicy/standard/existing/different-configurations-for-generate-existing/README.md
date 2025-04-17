## Description

This test ensures that a generate policy works as expected in case one rule sets the `generateExisting` field whereas the other don't set it. It is expected that rules which don't set the field will use the higher level value `spec.generateExisting`. 

## Expected Behavior

1. Create two Namespaces named `red-ns` and `green-ns`.

2. Create a policy with two generate rules:
    - The first rule named `generate-network-policy` matches Namespaces sets the `generateExisting` to `true`.
    - The second rule named `generate-config-map` matches Namespaces and it doesn't set the field. It is expected that the rule will use the `spec.generateExisting` value which is `false`.

3. It is expected that a NetworkPolicy will be generated for each Namespace whereas ConfigMaps will not be generated.

## Reference Issue(s)

N/A
