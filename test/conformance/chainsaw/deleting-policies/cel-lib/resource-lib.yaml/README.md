# DeletingPolicy - Resource Library Test

This test demonstrates the use of the **Resource Library** in a Kyverno `DeletingPolicy`. It verifies that a Pod is deleted only if a specific value exists in a `ConfigMap`.

---

## Test Description

- A `ConfigMap` named `clusterregistries` is created in the `default` namespace.
- A `Pod` named `example` is also created in the `default` namespace.
- A `DeletingPolicy` is applied that checks the value of `clusterregistries.data["registries"]`.
- If the value is `"enabled"`, the Pod is eligible for deletion.
- Kyverno’s Cleanup Controller will delete the Pod on the next schedule if the condition is met.

---

## Expected Outcome

After the policy is applied and enough time passes (`sleep 65s`), the Pod should be **deleted** if the ConfigMap's key `"registries"` is `"enabled"`.

Chainsaw will verify this by asserting that the Pod is no longer present in the cluster.
