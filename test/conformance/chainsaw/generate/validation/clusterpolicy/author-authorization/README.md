# Generate and cleanup policy author authorization

This test impersonates a service account that can manage legacy generate and
cleanup policies, but initially has no target permissions. Both policy creations
must fail with a policy-author authorization error even though their controllers
have the necessary permissions. Granting a namespace-scoped ConfigMap Role then
allows both creations: `get` and `create` for this unsynchronized generate rule,
and `list` and `delete` for cleanup.

The same two policies cover update compatibility. Metadata-only and unchanged
spec updates succeed without author target permissions; a changed generate
name or cleanup schedule is rejected.

Go tests retain the detailed authorization matrix:
`pkg/policy/generate/validate_test.go`, `reference_auth_test.go`, and
`clone_source_auth_test.go` cover target types, dynamic namespaces, every
cloneList kind, and separate source-read access. The update and controller-check
cases remain in `pkg/validation/policy/author_update_test.go` and
`generate_controller_order_test.go`; cleanup author/controller separation and
updates remain in `pkg/validation/cleanuppolicy/validate_test.go`.
