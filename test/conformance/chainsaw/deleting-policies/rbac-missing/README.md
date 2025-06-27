# Kyverno Chainsaw Test: DeletingPolicy Without RBAC

##  Purpose

This test validates the behavior of Kyverno's `DeletingPolicy` when the required RBAC permissions for cleanup are **not granted**. It ensures that:

- Kyverno does **not delete** resources when it lacks the necessary permissions.
- The cleanup controller fails gracefully.
- Resources remain intact, and this behavior is verifiable via Chainsaw assertions.

---

## Files

| File               | Description                                                                 |
|--------------------|-----------------------------------------------------------------------------|
| `chainsaw-test.yaml` | Chainsaw test definition with all steps                                    |
| `configmap.yaml`         | ConfigMap definition that will be targeted by the DeletingPolicy                  |
| `configmap-assert.yaml`  | Assertion file to confirm the ConfigMap's presence                                |
| `policy.yaml`      | A `DeletingPolicy` targeting the ConfigMap, but without RBAC                      |

> **Note**: This test intentionally omits any RBAC configuration for Kyverno's cleanup controller.

---

## Test Flow

| Step         | Action                                                           |
|--------------|------------------------------------------------------------------|
| `step-01`    | Creates the ConfigMap `rbac-missing-pod`                               |
| `step-02`    | Asserts that the ConfigMap exists                                      |
| `step-03`    | Applies the `DeletingPolicy` (with no RBAC)                      |
| `step-04`    | Waits for 65 seconds (policy runs every minute)                 |
| `step-05`    | Asserts again that the ConfigMap still exists (i.e., was **not** deleted) |

---

## Expected Behavior

- Kyverno **should not** be able to delete the ConfigMap.
- Chainsaw assertions in `step-05` should pass because the ConfigMap still exists.
- The test passes **only if the ConfigMap is not deleted**, proving that proper RBAC is mandatory for cleanup.

