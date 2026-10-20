## Description

This test tries to create policies with different combinations of unknown kind and/or subresource.
The policies should be accepted with a discovery warning identifying the unresolved kind.
Server-side dry runs check successful admission and the specific warning for each selector without persisting policies.
