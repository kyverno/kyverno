## Description

This test creates a policy, a policy exception and tries to create a couple configmaps.
The policy exception is configured to apply only to the `emergency` configmap and has wildcard in the rule name.
The `emergency` configmap is expected to create fine while other configmaps creations should fail.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above, configured to apply to configmap named `emergency`
1.  - Try to create a confimap named `emergency`, expecting the creation to succeed
    - Try to create a confimap named `foo`, expecting the creation to fail
