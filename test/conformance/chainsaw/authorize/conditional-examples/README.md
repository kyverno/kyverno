# AuthorizingPolicy CEL Examples

This directory mirrors the developer examples for the current CEL-based AuthorizingPolicy runtime.

## Activation Fields

The canonical activation field list is documented in [../../../../../docs/dev/conditional-authorization/DESIGN.md](../../../../../docs/dev/conditional-authorization/DESIGN.md) under "CEL Activation Shape".

In the authorization phase (`/authz/subjectaccessreview`), object labels are not injected.
In the conditions phase (`/authz/conditions`), object payload fields can populate `request.object`, `request.oldObject`, `request.resourceLabels`, and `request.oldResourceLabels`.

## Included Samples

- `tenant-governance-breakglass.yaml`: tenant namespace governance with conditional break-glass writes.
- `role-based-pod-access.yaml`: production workload controls including `pods/exec` conditional access.
- `api-surface-control.yaml`: non-resource path controls for health, metrics, and debug endpoints.
- `payload-aware-conditional.yaml`: condition-phase policy using labels and object payload fields.
