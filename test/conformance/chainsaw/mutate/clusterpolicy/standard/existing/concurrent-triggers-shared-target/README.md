## Description

Many trigger resources mutate the **same** target resource, and each patch is derived
from the target's current content. This is the shape that loses writes when the
background controller does not handle update conflicts: the workers race on one
object, one wins per resourceVersion, and the losers are dropped once the
UpdateRequest exhausts its requeues.

## Expected Behavior

Every trigger is registered on the shared target. The target's `registered` key
holds all twenty trigger names.

If conflicts are not retried — or are retried by replaying the previously computed
patch instead of recomputing it against the latest version of the target — the key
holds only a subset and the test fails.
