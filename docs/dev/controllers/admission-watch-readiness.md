# Admission watch readiness

## Problem and scope

[Issue #15626](https://github.com/kyverno/kyverno/issues/15626) reports admission replicas continuing to serve requests after their Kubernetes API watch streams failed. The reported delay before shutdown was approximately 4.5 minutes. This was an observed incident, not a fixed timeout or a bound on cache staleness.

The readiness monitor covers the admission controller's legacy policy and policy-exception watches, the controller-runtime watches used by CEL policy engines and their exceptions, and GlobalContextEntry definitions. It observes the streams used by those consumers, including their namespace and selector restrictions. It does not create a separate health-check watch as a substitute for monitoring them.

Dynamic resource watches created by GlobalContextEntry, external API polling, ConfigMap caches, and other auxiliary caches are outside this change. Monitoring GlobalContextEntry definitions therefore does not establish freshness of every entry's underlying data.

## Readiness behavior

Each monitored stream retains its own health state. An interruption starts a fixed 30-second grace period; repeated failures do not restart that period. A stream that remains interrupted past the grace period causes `/health/readiness` to fail alongside the existing readiness checks. Recovery of one stream does not clear another stream's failure.

Normal watch termination and replacement are allowed to recover within the same grace period. A healthy, idle watch does not become unhealthy merely because no objects changed. Kubernetes does not guarantee a periodic bookmark interval, so silence alone is not a failure signal. See [Kubernetes watch bookmarks](https://kubernetes.io/docs/reference/using-api/api-concepts/#watch-bookmarks).

Successful establishment of an ordinary resumed watch counts as restored watch connectivity. For a streaming initial list, recovery requires forwarding the initial-events-end bookmark; opening the stream alone is insufficient. Failures during list/watch establishment, watch error events, and stream termination remain observable. Client-go continues to own reconnection, relisting, and retry backoff.

This is a watch-connectivity readiness check. Recovery does not certify that every queued informer event, policy-cache update, or CEL compilation has finished. It does not provide a bound on all derived admission-state staleness.

Only readiness changes. Liveness, process lifetime, admission handlers, webhook failure policies, and Helm probe settings retain their existing behavior. The 30-second grace period is not the total endpoint-removal latency: kubelet probe scheduling and readiness propagation add delay. Requests already in flight or sent directly to a replica are not blocked by this monitor.

## Integration constraints

- Keep health state per actual informer/watch consumer, rather than sharing recovery state solely by resource kind.
- Preserve list/watch options and ordinary event delivery, and release monitoring goroutines when the informer stops.
- Monitor optional policy and exception resources only when their consumers are enabled and started.
- Do not use `HasSynced()` as ongoing watch health. It describes initial cache synchronization. Likewise, informer resync reprocesses cached objects and is not a fresh API list. See [client-go shared informer](https://github.com/kubernetes/client-go/blob/v0.36.4/tools/cache/shared_informer.go) and [reflector](https://github.com/kubernetes/client-go/blob/v0.36.4/tools/cache/reflector.go).
- Do not rely exclusively on `SetWatchErrorHandler`: the reflector handles some stream errors internally without returning an error to that callback. Observe the watch lifecycle itself. See [reflector watch handling](https://github.com/kubernetes/client-go/blob/v0.36.4/tools/cache/reflector.go).

Preserve the watchdog guards introduced by [PR #16434](https://github.com/kyverno/kyverno/pull/16434), which prevent policy readiness downgrades and empty webhook publication while webhook health is unconfirmed. The separate two-informer PolicyException race in [issue #16989](https://github.com/kyverno/kyverno/issues/16989), addressed by [PR #16993](https://github.com/kyverno/kyverno/pull/16993), is not solved by watch readiness.

## Verification scenarios

Run the deterministic tracker, HTTP list/watch, controller-runtime cache, and readiness regressions with:

```sh
go test -race ./pkg/informers/health ./pkg/utils/runtime ./cmd/kyverno
```

`TestAdmissionWatchReadinessRegression` exercises both ordinary LIST plus watch EOF and watch-list plus an error event. It verifies HTTP 500 during the interruption and eventual policy evaluation against updated state after recovery. The scenarios below describe acceptance coverage rather than a claim about every possible network failure.

Use fake time and controlled list/watch responses to verify:

- Healthy and idle streams remain ready; transient interruptions recover within the grace period.
- List failures, watch-establishment failures, error events, and closed streams cause readiness failure after 30 seconds, including when retries keep failing.
- Recovery occurs in place without a restart; independent failures remain unhealthy until each stream recovers.
- Streaming initial-list recovery waits for the initial-events-end bookmark to be forwarded.
- Existing readiness failures still fail readiness, while watch failures do not change liveness.
- Namespace/selector options, optional resource registration, normal event forwarding, and cancellation are preserved; race-enabled tests detect unsafe concurrent state access.

For a cluster-level reproduction, use a disposable multi-replica installation and a controllable API proxy for one admission replica. Establish healthy policy watches, then terminate and block the selected replica's monitored watches while allowing short API calls. Change a policy or exception through an unaffected connection. Verify that the affected replica's readiness fails after the grace period and probe delay, while healthy replicas remain ready and the affected process stays alive. Restore watch connectivity and verify that the replica becomes ready without restarting. Separately verify eventual enforcement of the updated policy or exception; do not infer completed policy compilation from the readiness transition alone. Repeat with a GlobalContextEntry definition update, and retain the dynamic-data scope limitation above.
