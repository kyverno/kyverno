## Description

This test verifies the resource validation webhook is configured correctly when a policy targets all `*/scale` subresources.

The expected webhook list also includes the independent `protect-generation-labels.kyverno.svc` webhook.
Policy routing assertions retain the exact list length and ordering used by the controller.

## Steps

1.  - Create a policy targeting `*/scale`
    - Assert policy gets ready
1.  - Assert that the resource validation webhook is configured correctly
