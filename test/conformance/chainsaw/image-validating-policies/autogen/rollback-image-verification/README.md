# Description

CEL equivalent of the legacy `verify-images/clusterpolicy/standard/rollback-image-verification`
chainsaw test (deleted whole by #18164; last present at commit `7b1ed44f3^`), migrated for #17707.

This test verifies that an `ImageValidatingPolicy` autogen'd onto `Deployment` continues to accept
an already-verified, digest-mutated image across a `kubectl rollout undo`. A rollback is an `UPDATE`
to the Deployment, not a fresh `CREATE`; this proves the autogen'd rule's operation set, and digest
mutation/verification, both apply on that update path the same way they did on the original creates.

# Expected Behavior

Two Deployment revisions are created in turn, each image-verified and digest-mutated on admission.
After `kubectl rollout undo`, the Deployment's pod template reverts to the first revision's
already-verified, digest-pinned image, and the rollback itself is not denied.

# Reference Issue(s)

5363 (original legacy test), #17707 (CEL migration)
