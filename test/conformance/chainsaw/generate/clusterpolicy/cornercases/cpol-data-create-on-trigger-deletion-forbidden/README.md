## Description

This is a corner case test to ensure a generate data rule triggered on the deletion of the trigger resource still executes when the background controller is not allowed to read the trigger kind.

The trigger is a `PodTemplate`, a kind the background controller has no RBAC permissions for. When verifying that the deletion was persisted, the background controller's `get` on the trigger returns `403 Forbidden`. The check must be skipped and the deleted object must drive generation, instead of failing the UpdateRequest on every retry.

## Expected Behavior

The test first asserts that the background controller cannot `get` PodTemplates. After the trigger is deleted, the downstream ConfigMap must be created with the deleted trigger's name in its data. If it is not created, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/17822
