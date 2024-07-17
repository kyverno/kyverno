## Description

This test validates the use of parameter resources in validate.cel subrule.

This test creates the following:
1. A namespaced custom resource definition `NameConstraint`
3. A policy that checks the namespace name using the parameter resource.
4. A namespace `testing`.

## Expected Behavior

Since the parameter resource is namespaced-scope and the policy matches cluster-scoped resource `Namespace`, therefore the creation of a namespace is blocked
