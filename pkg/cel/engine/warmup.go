package engine

import (
	"context"
	"time"

	"github.com/kyverno/kyverno/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// WarmUp synchronously reconciles the given policy keys so that their compiled form is available
// before admission traffic is served. mgr.GetCache().WaitForCacheSync() only guarantees that the
// raw objects are in the cache, not that the controller workqueue has processed them.
//
// A failure to reconcile a single policy is logged and skipped rather than returned: one policy that
// no longer compiles (e.g. after an upgrade tightened the CEL environment) must not prevent the
// admission controller from starting. Such a policy is not enforced either way, and the controller
// retries it through its normal workqueue. Only context cancellation is returned as an error.
func WarmUp(ctx context.Context, kind string, r reconcile.Reconciler, keys []client.ObjectKey) error {
	logger := logging.WithName("cel-warmup").WithValues("kind", kind)
	start := time.Now()
	failed := 0
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err != nil {
			failed++
			logger.Error(err, "failed to warm up policy, leaving it to the controller", "policy", key.String())
		}
	}
	logger.Info("warm-up completed", "policies", len(keys), "failed", failed, "duration", time.Since(start).String())
	return nil
}
