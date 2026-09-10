# AGENTS.md — pkg/background

Processes `UpdateRequest` (UR) objects created by `pkg/webhooks` for `generate` and `mutateExisting` rules. No
package doc comment exists — this file is the closest thing to one.

## DeletingPolicy (`dpol`) has NO presence here — a common wrong assumption

There is no `pkg/background/dpol` and no reference to `DeletingPolicy` anywhere in this package. DeletingPolicy
execution lives entirely in `pkg/controllers/deleting`, a normal informer-driven controller whose `reconcile`
computes the policy's next execution time from its schedule and **re-queues itself** via
`queue.AddAfter(key, delay)` (clamped to a 1s minimum to avoid hot-looping) — a self-scheduling loop, not a
Kubernetes `CronJob` resource, and it never creates or consumes `UpdateRequest` objects. Don't go looking for dpol
background processing here; it doesn't exist by design.

## Trigger mechanism: an informer-driven controller, not something invoked directly

`update_request_controller.go` watches `UpdateRequest`s via an informer and only processes ones with
`Status.State == Pending`. It dispatches on `ur.Spec.GetRequestType()` into four separate sub-controllers:
`mutate` (legacy mutateExisting), `generate` (legacy), `gpol` (CEL GeneratingPolicy), `mpol` (CEL MutatingPolicy).
After processing, it **re-fetches the UR live** (not from the informer cache) before updating status — `Completed`
deletes the UR, `Failed` resets it to `Pending` for retry. This live re-fetch specifically avoids a race where a
completed UR would never get deleted.

## Legacy generate/mutate vs. CEL gpol/mpol are structurally different, not just newer

Legacy controllers re-fetch the `PolicyInterface` and run it through the shared JMESPath `engine.PolicyContext` +
`Engine.Generate/Mutate`. The CEL paths go through the CEL engines built on `pkg/cel`, use `libs.Context` for CEL
global context, and **gpol additionally owns a process-wide, in-memory `WatchManager`** (mutex-protected maps of
dynamic watchers, policy refs, ref counts) for downstream drift-detection that legacy generate doesn't have — this
state is not persisted; on restart, watches rebuild from the policy's already-generated resources. gpol's
watcher-sync/stale-downstream-cleanup runs in a fire-and-forget goroutine **after** the UR is already marked
complete, so a sync failure there is only logged, never reflected in UR status.

## Trigger resolution deliberately never trusts the admission payload

`common.GetResource` looks up the trigger resource **live, by UID** — if the UID isn't found among live resources,
it refuses to fall back to the UR's recorded admission-request payload and errors instead
(see https://github.com/kyverno/kyverno/issues/16566): a rejected or superseded admission request must not drive
policy evaluation. Delete-operation triggers are verified similarly — the controller re-queries the live cluster to
confirm the deletion actually persisted (and wasn't itself rejected by another webhook) before treating the
resource as gone. Update-operation triggers are always resolved from live state too, never the admission payload.
If you're debugging "why did my UpdateRequest silently fail to find its trigger," this UID/liveness discipline is
almost always why — it's intentional, not a bug to route around.

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
