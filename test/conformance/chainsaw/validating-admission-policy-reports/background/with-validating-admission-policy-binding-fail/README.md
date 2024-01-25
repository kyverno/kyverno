## Description

This test checks that policy reports are generated successfully as a result of applying the ValidatingAdmissionPolicy with its binding to a resource.

## Steps

1.  - Create a `staging-ns-2` namespace whose label is `environment: staging-ns-2`
1.  - Create a Deployment named `deployment-3` with 7 replicas in the `staging-ns-2` namespace.
1.  - Create a ValidatingAdmissionPolicy that checks deployment replicas to be less than or equal to 5.
    - Create a ValidatingAdmissionPolicyBinding that matches resources whose namespace has a label of `environment: staging`.
1.  - A policy report is generated for `deployment-3` with a fail result.
