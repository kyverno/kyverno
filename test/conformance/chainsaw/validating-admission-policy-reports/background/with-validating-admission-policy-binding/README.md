## Description

This test checks that policy reports are generated successfully as a result of applying the ValidatingAdmissionPolicy with its binding to a resource.

## Steps

1.  - Create a `staging-ns` namespace whose label is `environment: staging`
1.  - Create a Deployment named `deployment-1` with 7 replicas in the `default` namespace.
    - Create a Deployment named `deployment-2` with 3 replicas in the `default` namespace.
    - Create a Deployment named `deployment-3` with 7 replicas in the `staging-ns` namespace.
    - Create a Deployment named `deployment-4` with 3 replicas in the `staging-ns` namespace.
1.  - Create a ValidatingAdmissionPolicy that checks deployment replicas to be less than or equal to 5.
    - Create a ValidatingAdmissionPolicyBinding that matches resources whose namespace has a label of `environment: staging`.
1.  - A policy report is generated for `deployment-3` with a fail result.
    - A policy report is generated for `deployment-4` with a pass result.
    - No policy reports generated for both `deployment-1` and `deployment-2`.
