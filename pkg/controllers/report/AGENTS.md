# AGENTS.md — pkg/controllers/report

Reporting subsystem that watches cluster resources, runs background policy scans, and aggregates intermediary reports into final `PolicyReport`/`ClusterPolicyReport` (or OpenReports `Report`/`ClusterReport`) objects.

## Subpackage responsibilities

| Subpackage | Controller / Role | Workers (default) | Key state |
|---|---|---|---|
| `resource/` | `resource-report-controller` | `1` | `MetadataCache` (`dynamicWatchers` map of GVR → active watch + `hashes` map by `types.UID`, guarded by `sync.RWMutex`), plus `watchDeathChan` for restarting dead watches |
| `background/` | `background-scan-controller` | `2` | Consumes `resource.MetadataCache` events (ignores `Deleted`; queues `<uid>` or `<namespace>/<uid>` after `enqueueDelay = 30s`), evaluates policies via `utils.Scanner`, writes `EphemeralReport`/`ClusterEphemeralReport` |
| `aggregate/` | `aggregate-report-controller` | `10` | Two-queue (`frontQueue`, `backQueue`) aggregator from ephemeral reports to permanent reports; maintains `reportUUIDToPolicyCache` guarded by `cacheMu` (pruned every 10s by a background goroutine) |
| `utils/` | Shared helpers (not a controller) | n/a | `Scanner` (`scanner.go`), `ProviderCache` (`provider_cache.go`), report/policy lister helpers (`utils.go`), and event generation (`events.go`) |

## Background controller & scanner invariants

- **Shared controller-lifetime cache**: `background.NewController` constructs one `utils.ProviderCache` and injects it into `utils.NewScannerWithCache(...)` so compiled CEL policy providers (`vpol`, `mpol`, `ivpol`) are reused across scan sweeps and across all background worker goroutines. `utils.NewScanner(...)` (and `NewScannerWithCache(..., nil)`) creates a standalone cache instance.
- **Policy deletion eviction**: `deleteVP`, `deleteMP`, and `deleteIVP` in `background/controller.go` call `c.providerCache.Evict(obj.GetKind(), obj.GetNamespace(), obj.GetName())` before calling `c.enqueueResources()`.
- **Periodic re-arming & diffing**: `background.reconcile` defers `c.queue.AddAfter(key, c.forceDelay)` so periodic re-scans stay armed even on transient errors. `needsReconcile` compares resource hashes and policy/binding `ResourceVersion` labels to decide between a full scan, a partial policy re-evaluation, or a no-op.
- **CEL cost budget**: `scanner.ScanResource` calls `requestbudget.EnsureBackground(ctx)` once per scanned resource so every policy evaluated against that resource shares the background CEL cost budget.
- **Report result normalization**: `filterMatchConditionSkips` drops `RuleStatusSkip` entries whose `SkipReason` is `SkipReasonMatchConditions` for `ValidatingPolicy` scans. For `MutatingPolicy` scans, a passing rule with no exceptions whose `PatchedResource` differs semantically from the scanned resource (`!equality.Semantic.DeepEqual`) is rewritten to `RuleFail("", engineapi.Mutation, "mutation is not applied", nil)`.

## `ProviderCache` concurrency and invalidation contract

- **Per-key serialization without global compile locking**: Concurrent background workers calling `GetValidatingPolicyProvider`, `GetMutatingPolicyProvider`, or `GetImageValidatingPolicyProvider` share `providerStore[T]`:
  - Fast-path cache hits acquire only `lock.RLock()` and do not contend on per-key locks.
  - Cache misses look up or insert a per-key `cacheSlot[T]` under `lock.Lock()`, release `lock`, acquire `slot.mu.Lock()`, re-check `slots[key] == slot` and `slot.entry` under `lock.RLock()`, compile *without* holding `lock`, and publish under `lock.Lock()`.
  - **Strict lock ordering**: `slot.mu -> lock`. Never acquire `slot.mu` while holding `lock`, and never hold `lock` across CEL compilation. Unrelated policy keys, fast-path hits on other keys, `Evict`, and `Clear` never block on an in-flight compilation.
- **No stale overwrites or resurrection on `Evict`/`Clear`**: `Evict` and `Clear` detach slots from `slots` under `lock.Lock()`. If a slot is detached while a compilation is waiting or in flight, `tryFillSlot` detects `slots[key] != slot`:
  - Before compiling, it unlocks `slot.mu` and retries against the current map entry.
  - After compiling, it skips storing `slot.entry` so a deleted/evicted policy is never resurrected and an older in-flight compile cannot overwrite a newer entry.
- **Cache key & invalidation**: Entries are keyed by `kind/namespace/name` (or `kind/name` when cluster-scoped) and validated against `(policyRevision, matchedExceptionsKey)`. Revision prefers `ResourceVersion`, falling back to `gen:<Generation>` and then a SHA-256 JSON hash. `PolicyException` fingerprints filter on `(ref.Kind, ref.Name)` and sort matching exception revisions, so unrelated `PolicyException` changes do not invalidate cached providers.
- **Testing hooks**: `HookVpolCompiled`, `HookMpolCompiled`, and `HookIvpolCompiled` are package-level globals invoked inside the serialized compile callback (only on actual cache-miss compilations). Unit tests that assign these hooks must not call `t.Parallel()`.
