## Description

This test tries to create a policy targeting the `Scale` kind.
The `Scale` kind doesn't map to a top level resource, so the policy is accepted with a discovery warning.
A server-side dry run checks successful admission and the specific warning without persisting the policy.