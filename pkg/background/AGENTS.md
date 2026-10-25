# AGENTS.md — pkg/background

Processes `UpdateRequest` (UR) objects created by `pkg/webhooks` for the CEL policy types `gpol`
(GeneratingPolicy) and `mpol` (MutatingPolicy) background/mutate-existing requests — see the dispatch section
below for how `update_request_controller.go` routes between the two. Legacy `kyverno.io/v1` `generate`/
`mutateExisting` execution was removed from this binary in #17710 PR 2: a pre-existing legacy `mutate`/`generate`
UR left over from an older release is deleted on its first sync here, never processed — see `legacy.go` and the
dispatch section below. No package doc comment exists — this file is the closest thing to one.

## DeletingPolicy (`dpol`) has NO presence here — a common wrong assumption

There is no `pkg/background/dpol` and no reference to `DeletingPolicy` anywhere in this package. DeletingPolicy
execution lives entirely in `pkg/controllers/deleting`, a normal informer-driven controller whose `reconcile`
computes the policy's next execution time from its schedule and **re-queues itself** via
`queue.AddAfter(key, delay)` (clamped to a 1s minimum to avoid hot-looping) — a self-scheduling loop, not a
Kubernetes `CronJob` resource, and it never creates or consumes `UpdateRequest` objects. Don't go looking for dpol
background processing here; it doesn't exist by design.

## Trigger mechanism: an informer-driven controller, not something invoked directly

`update_request_controller.go` watches `UpdateRequest`s via an informer. Before the `Status.State == Pending`
gate, it checks `ur.Spec.GetRequestType()` for the legacy `Mutate`/`Generate` types: if found, it calls
`dropLegacyUpdateRequest` (`legacy.go`), which deletes the UR unconditionally — regardless of its status state —
and logs at `V(2)`, instead of processing it. This runs on every sync, not just once on upgrade, so any legacy UR
that reappears (e.g. recreated by an operator who hasn't noticed the binary no longer has a producer for it) is
deleted again. Only once that check falls through does the controller process `Status.State == Pending` URs,
dispatching on `ur.Spec.GetRequestType()` into one of the two CEL sub-controllers: `gpol` (CEL GeneratingPolicy),
`mpol` (CEL MutatingPolicy); an unrecognized type sets the UR `Failed` with a descriptive message instead of
silently dropping it. After processing, it **re-fetches the UR live** (not from the informer cache) before
updating status — `Completed` deletes the UR, `Failed` resets it to `Pending` for retry. This live re-fetch
specifically avoids a race where a completed UR would never get deleted.

An empty live state is the create-without-Pending case (#18194): create and the first `Pending` status write
are separate calls, and a failed status write used to leave the request unprocessed forever while it still
counted toward `updateRequestThreshold`. That sync does not run `processUR`. Once the live object is older
than `emptyStateRecoveryDelay` (5s, past `generator.applyResource`'s 3s retry of the initial status write),
reconcile sets `Pending` and the status update requeues the normal path. A younger empty object is left
unchanged and requeued for the rest of that delay, so the creator's in-flight `UpdateStatus` is not
overwritten (a conflict there makes the caller submit a second request). The decision uses the fresh read,
so a newer `Pending`, `Skip`, `Failed`, or `Completed` state is not repaired from a stale empty informer
copy. Legacy mutate/generate requests never reach this recovery; they are deleted above.

## CEL gpol/mpol are the only policy execution paths left here

The legacy `generate`/`mutateExisting` controllers (which re-fetched the `PolicyInterface` and ran it through the
shared JMESPath `engine.PolicyContext` + `Engine.Generate/Mutate`) were deleted from this package in #17710 PR 2;
`pkg/background/generate` itself (its only remaining caller) was deleted in #17710 PR 4, along with the
legacy-only helpers in `pkg/background/common` (`ManageLabels`, `MutateLabelsSet`, `PolicyInfo`, `TriggerInfo`,
`TagSource`, `FindDownstream`). `pkg/background/common` now holds only the gpol/UR helpers: `GetResource`,
`GetTrigger` and its `getTriggerFor*` helpers, `GenerateLabelsSet`, `UpdateStatus`,
`ResourceSpecFromUnstructured`. The CEL paths go through the CEL engines built on `pkg/cel`, use `libs.Context` for CEL global
context, and **gpol additionally owns a process-wide, in-memory `WatchManager`** (mutex-protected maps of dynamic
watchers, policy refs, ref counts) for downstream drift-detection — this state is not persisted; on restart,
watches rebuild from the policy's already-generated resources. gpol's watcher-sync/stale-downstream-cleanup runs
in a fire-and-forget goroutine **after** the UR is already marked complete, so a sync failure there is only
logged, never reflected in UR status.

## Trigger resolution prefers live state, but the admission payload isn't fully off-limits

This applies to CEL `gpol`, which calls `common.GetTrigger`/`common.GetResource`
(`pkg/background/common/resource.go`). `mpol` doesn't use this machinery at all (its processor never imports
these functions) and isn't covered by anything below.

`common.GetResource` looks up the trigger resource **live, by UID** — if the UID isn't found among live resources,
it refuses to fall back to the UR's recorded admission-request payload and errors instead
(see https://github.com/kyverno/kyverno/issues/16566): a rejected or superseded admission request must not drive
policy evaluation. Delete-operation triggers are verified similarly — the controller re-queries the live cluster to
confirm the deletion actually persisted (and wasn't itself rejected by another webhook) before treating the
resource as gone.

That said, two real fallbacks to the admission payload exist, and neither is a bug:
- `GetResource`'s final branch (after the UID and name lookups both fall through) decodes
  `AdmissionRequest.Object.Raw`/`OldObject.Raw` directly when the resource spec has **neither** a UID **nor** a
  name — this is reachable from `getTriggerForUpdateOperation`, so "update triggers never use the payload" isn't
  quite right; it only holds once the trigger has an identity to look up live.
- `getTriggerForCreateOperation` explicitly reverts to `admissionutils.ExtractResources` on the admission payload
  when no live trigger is found *and* the request has a non-empty `SubResource` — a deliberate exception for
  subresource requests, not the UID-fallback path above.

If you're debugging "why did my UpdateRequest silently fail to find its trigger" (or, conversely, why it used
payload data you didn't expect), this UID/liveness discipline plus these two named exceptions is almost always
why — check which branch of `GetResource`/`getTriggerForCreateOperation` your case actually hit before assuming
either a bug or a blanket "never trusts the payload" rule.

## `mpol` has its own backward-compatibility shim worth knowing before touching UR key format

Old `UpdateRequest`s stored a bare policy name for `NamespacedMutatingPolicy`; newer ones store `namespace/name`.
`mpol.GetPolicy` tries the cluster-scoped lookup first and falls back to the namespaced one for old-format keys —
don't "clean up" this fallback without checking for URs created by older Kyverno versions still in the cluster.
mpol also skips no-op background re-scans (already-compliant targets) without hitting the API server, but still
records the event/report for them.

## This is the most actively-changing part of the CEL policy stack right now

Recent history (`18ed60efc`, `f6718f20a`, `8579ea3f6`, `5e144c6e8`, `e6ec4c2de`, `0b8c53a46`) is dominated by
gpol/mpol downstream-cleanup and trigger-staleness fixes — read these before assuming the current behavior is
stable; this area has had several correctness bugs in quick succession, mostly around stale-downstream deletion and
trigger-UID validation (the exact invariants documented above).
