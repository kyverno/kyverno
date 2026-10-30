# AGENTS.md — pkg/globalcontext/externalapi

Manages cached `GlobalContextEntry` resources whose source data is fetched from external HTTP endpoints or Kubernetes API calls (`gce.Spec.APICall`). Lives under `pkg/globalcontext/externalapi` and is instantiated by `pkg/controllers/globalcontext/controller.go`.

## Lifecycle & Polling Architecture

Each external API entry wraps an asynchronous polling loop initialized via `New(...)` and managed by a `wait.Group`:

1. **Background Polling Loop**:
   - Polling starts in a dedicated goroutine managed by `wait.Group.StartWithContext(ctx, ...)`.
   - Runs `wait.UntilWithContext(ctx, func(ctx context.Context) { ... }, period)` according to `gce.Spec.APICall.RefreshInterval`.
   - In each tick:
     - `doCall`: Executes the configured HTTP or API request using `apicall.Executor` with exponential retry backoff (`retry.OnError`) bounded by `gce.Spec.APICall.RetryLimit`.
     - `setData`: When `doCall` succeeds, raw response bytes are parsed into JSON (`json.Unmarshal`) and passed through any configured JMESPath projections (`projection.JP.Search(jsonData)`).
     - Success / Failure branching:
       - **On Failure** (network/API timeout, bad HTTP status, malformed response payload, or projection search error):
         - Records the error in `e.err` and skips updating `e.dataMap`.
         - Emits a Kubernetes error event (`entryevent.NewErrorEvent`) referencing the `GlobalContextEntry`.
         - When `shouldUpdateStatus` is true, calls `updateStatus(ctx, gce, kyvernoClient, false, err.Error())`.
       - **On Success** (valid payload and all projections computed):
         - Stores projected and raw results in `e.dataMap`, clears `e.err`.
         - When `shouldUpdateStatus` is true, calls `updateStatus(ctx, gce, kyvernoClient, true, "Ready")`.

## Critical Controller Invariants

### 1. Initialization and Projection Parsing Must Update Status on Failure
In `New(...)`, JMESPath queries for all projections in `gce.Spec.Projections` are pre-compiled via `jp.Query(p.JMESPath)`.
- If query parsing fails or `jp` is nil, `New` **must not** return without status reporting.
- Before returning `nil, err`, `New` must:
  - Log the error.
  - Emit an error event via `eventGen.Add(...)`.
  - Update status via `updateStatus(ctx, gce, kyvernoClient, false, err.Error())` if `shouldUpdateStatus` is true.
  - Ensure background goroutines are never spawned and contexts are not leaked.

### 2. Status Condition & Refresh Time Transitions
Status updates are managed by `updateStatus`:
- Uses `retry.RetryOnConflict(retry.DefaultRetry, ...)` and fetches the latest object version (`latest, err := kyvernoClient.KyvernoV2beta1().GlobalContextEntries().Get(...)`) to prevent lost updates due to Kubernetes resource contention.
- **Success (`ready = true`)**:
  - Condition `Ready`: `Status = ConditionTrue`, `Reason = Succeeded`, `Message = "Ready"`.
  - `LastRefreshTime`: Updated to `metav1.Now()`.
- **Failure (`ready = false`)**:
  - Condition `Ready`: `Status = ConditionFalse`, `Reason = Failed`, `Message = <error string>`.
  - `LastRefreshTime`: **Must not be updated** on failure; preserves the last known successful refresh timestamp.

### 3. Graceful Shutdown & Goroutine Teardown
- `e.Stop()` signals the polling context via `cancel()` and blocks on `group.Wait()` until the worker exits cleanly.
- Wrapped in a `sync.Once` (`stopOnce.Do(...)`) so concurrent or repeated `Stop()` calls are idempotent and never deadlock or panic.
- Lock ordering: `Stop()` does not acquire `e.Lock()`, preventing deadlocks between ongoing `Get()` / `setData()` operations and shutdown.
