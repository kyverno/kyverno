## Description

This test tries to create various policies targeting subresources and `background` set to `true`.

## Expected Behavior

Unresolved kinds (`Scale` and the nonexistent `Pod/scale`) are accepted with a discovery warning.
Server-side dry runs check both successful admission and the specific warning without persisting those policies.

Resolvable subresources (`apps/v1/Deployment/scale`) and wildcard subresource selectors
(`Pod/*`, `*/*`, and `*/status`) must be rejected specifically because background scanning is enabled.
