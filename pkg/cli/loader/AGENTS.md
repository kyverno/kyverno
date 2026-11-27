# AGENTS.md — pkg/cli/loader

This package implements concurrent cluster resource loading for the Kyverno CLI (`ClusterLoader` and `WorkerPool`), fetching Kubernetes resources matching policy match constraints with configurable batch sizes, timeouts, and concurrency.

## Architecture and Execution Flow

1. **Loader Initialization (`NewClusterLoader`, `LoadResourcesConcurrent`)**:
   - `LoadResourcesConcurrent` creates a `ClusterLoader` with the specified dynamic client and `ResourceOptions`.
   - Bounded by a parent context whose cancellation is deferred through `resourceLoader.Close(cancel)`.

2. **Task Creation (`createLoadingTasks`)**:
   - Discovers API group/version/resource mappings via the client REST mapper.
   - Builds independent `LoadTask` specifications configured with paginated list limits (`ListOptions.Limit = BatchSize`) and namespace constraints.

3. **Task Execution and Result Collection (`executeTasks`, `WorkerPool`)**:
   - Tasks are dispatched to `WorkerPool.SubmitTask`.
   - Results are streamed through `WorkerPool.GetResults()`.
   - `executeTasks` collects results using a reusable `time.Timer` initialized with `resourceOptions.Timeout`.

## Worker Pool Lifecycle and Shutdown Contract

- **Worker Context and Cancellation**:
  - `WorkerPool` workers run under a managed cancellable context (`wp.ctx`, `wp.cancel`).
  - Active Kubernetes list operations (`task.Client.List(ctx, ...)`) receive this context and are immediately aborted when cancellation is triggered.
- **Graceful, Idempotent, and Race-Safe Shutdown**:
  - `WorkerPool.Close(cancel)` is guarded by `sync.Once`: it triggers caller cancellation (`cancel()`), cancels the worker context (`wp.Cancel()`), marks the pool closed under `wp.mu` before closing `taskQueue`, waits for in-flight worker goroutines to exit (`wp.wg.Wait()`), and closes `resultChan`.
  - `SubmitTask` holds `wp.mu.RLock()` while checking `wp.closed`, caller `ctx.Done()`, and `wp.ctx.Done()` (safely handling nil `wp.ctx` in test-constructed pools) so concurrent `SubmitTask` and `Close` calls cannot panic or race.
  - Because worker contexts are cancelled, workers blocked on network requests in `task.Client.List` do not hang indefinitely during shutdown.

## Timeout and Partial Result Contract

- **Caller Timeout & Timer Reuse**:
  - `NewClusterLoader` preserves positive caller-supplied `ResourceOptions.Timeout` values and defaults non-positive values (`<= 0`) to `5 * time.Minute`.
  - Result collection in `executeTasks` uses a single `time.Timer` reset upon each received result. This bounds silence between consecutive results without accumulating timer allocations.
- **`ContinueOnError` Behavior**:
  - When `ContinueOnError` is `false`: A timeout while awaiting task results immediately returns an error (`timeout waiting for task results`).
  - When `ContinueOnError` is `true`: A timeout breaks the result collection loop immediately via a labeled break (`break CollectResults`), returning all partial results collected up to that point rather than waiting for remaining missing tasks sequentially.
