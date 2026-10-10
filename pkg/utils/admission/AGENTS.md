# AGENTS.md — pkg/utils/admission

Helpers for decoding and inspecting `AdmissionReview` payloads. Most of this package unmarshals
into typed Kyverno APIs; two files deliberately do not.

## The type-free boundary

`metadata.go` and `legacy.go` decode into `metav1.PartialObjectMetadata` only. They must never
import a Kyverno policy type, because they have to keep working after #17713 deletes
`api/kyverno/v1` and the generated clients. When you add a helper for the legacy denial path, follow
`metadata.go`'s shape rather than `policy.go`'s.

## `IsFinalizerRemovalOnTerminatingObject` is fail-closed

This is the one permitted update on a legacy policy kind once all other writes are denied, so it is
an explicit exception inside a deny-everything rule. Treat every condition as load-bearing:

- The request must be a top-level UPDATE with no subresource.
- `old.deletionTimestamp` must be non-nil. Clients cannot set that field themselves, so the
  carve-out cannot be forged without first going through the already-allowed DELETE path.
- The new finalizer set must be a strict subset of the old one.
- Everything else must be byte-identical, compared as opaque JSON after zeroing only the
  apiserver-managed fields that legitimately change.

Anything that fails a check is not a finalizer removal and the caller denies it. A decode failure is
reported as `(false, err)`, never `(false, nil)`.

**Guard every decoded pointer before dereferencing it.** `UnmarshalPartialObjectMetadata` returns
`(nil, nil)` for a JSON `null` body, since it unmarshals into a pointer. Skipping that check panics
in the admission path instead of denying, which is how this contract was broken once already.

Do not relax any condition to make a recovery case easier. The documented recovery path is
`kubectl patch --type=json` removing a finalizer and nothing else; widening this function reopens a
general-purpose update bypass.

Related: [`pkg/deprecations`](../../deprecations/AGENTS.md), [`pkg/webhooks`](../../webhooks/AGENTS.md).
