## Description

This test makes sure that a generate existing policy that matches wildcard names in the `match` block works as expected. The policy should only generate resources for the existing resources that match the wildcard name.


## Expected Behavior

1. Create two Namespaces: `tst-home-dev` and `tst-mobile-dev`.

2. Create a policy that generates a ServiceAccount for all existing namespaces whose name matches the wildcard `tst-*`.

3. Two ServiceAccounts are generated in `tst-home-dev` and `tst-mobile-dev`.

## Reference Issue(s)

#10886
