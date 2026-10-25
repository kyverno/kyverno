# Global context entries

Runtime entries use `kyverno.io/v2` types. The controller in
`pkg/controllers/globalcontext` supplies the v2 informer and lister;
`externalapi.updateStatus` reads and writes through the v2 client.
For API lifecycle rules, use [the versioning guide](../../docs/context/shared/api-versioning.md).

- `store.Set` and `store.Delete` release the store lock before stopping an old
  entry. Keep teardown outside that lock: `Stop` can wait for background work.
- Kubernetes entries cancel their informer on watch errors. The error handler
  must not call `Stop`, which waits for that same informer and would deadlock.
  Initial cache sync is bounded by the caller's context and a 30-second timeout.
- Kubernetes entries protect cached projections with `projectedMu`; raw objects
  come from the informer lister. External API entries protect their data and
  error together with a mutex and publish projections only after all succeed.
- Entry teardown is safe to call repeatedly. Preserve cancellation before waiting
  for workers, including constructor failure paths.

Run the affected tests under `pkg/globalcontext` and
`pkg/controllers/globalcontext` when changing these contracts.
