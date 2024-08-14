## Description

This test ensures that a generate policy works as expected in case the rules have a different value for the `generateExisting` field.

## Expected Behavior

1. Create two Namespaces named `red-ns` and `green-ns`.

2. Create a policy with two generate rules:
    - The first rule named `generate-network-policy` matches Namespaces sets the `generateExisting` to `true`.
    - The second rule named `generate-config-map` matches Namespaces sets the `generateExisting` to `false`.

3. It is expected that a NetworkPolicy will be generated for each Namespace whereas ConfigMaps will not be generated.

## Reference Issue(s)

N/A
