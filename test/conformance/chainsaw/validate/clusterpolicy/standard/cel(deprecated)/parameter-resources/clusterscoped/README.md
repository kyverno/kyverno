## Description

This test validates the use of parameter resources in validate.cel subrule.

This test creates the following:
1. A cluster-scoped custom resource definition `NamespaceConstraint`
3. A policy that checks the namespace name using the parameter resource.
4. Two namespaces.

## Expected Behavior

The namespace `testing-ns` is blocked, and the namespace `production-ns` is created.
