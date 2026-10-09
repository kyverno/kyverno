# Generate policy author authorization

This test impersonates a service account which may create legacy generate and
cleanup policies but initially has no permissions on their targets. It proves
that admission checks the policy author independently from the Kyverno
controllers.

The test also permits metadata-only and no-op updates by a policy reconciler
without target permissions, while rejecting changes to generate targets and
cleanup schedules. Only author authorization is skipped when the entire policy
spec is unchanged; controller permissions and structural validation still run.
A supplied empty author identity is rejected.

Creators and reconcilers that change a policy spec need target permissions:
`get` and `create` for generate rules, plus `update` and `delete` when
`synchronize: true`; cleanup policies require `list` and `delete`. This also
applies to GitOps, Helm, and operator service accounts. Reapplying an unchanged
spec does not require those additional author permissions.

The test covers static targets, variable target namespaces, variable target
kinds in direct and foreach generation, every cloneList kind, and the
CleanupPolicy sibling path. A namespace-scoped Role is sufficient for a static
target in that namespace. A variable target namespace requires cluster-wide
authorization because it can resolve to any namespace. Live admission requires
static target API versions and kinds; offline CLI apply/test retains support
for templates while still validating the structure of generate rules.

This regression intentionally covers `kyverno.io` Policy and ClusterPolicy.
CEL GeneratingPolicy and NamespacedGeneratingPolicy admission returns through
`pkg/cel/policies/gpol` before the legacy policy validator is called, and uses
its separate CEL authorizer model. The legacy SubjectAccessReview checks added
here therefore do not apply to CEL generating policies; their behavior remains
covered by the existing GPOL compiler and engine tests.
