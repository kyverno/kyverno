package resource

import (
	context "context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.uber.org/multierr"
	k8s_io_api_policy_v1 "k8s.io/api/policy/v1"
	k8s_io_client_go_kubernetes_typed_policy_v1 "k8s.io/client-go/kubernetes/typed/policy/v1"
)

func WithLogging(inner k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface {
	return &withLogging{inner, logger}
}

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface, client, kind string) k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface {
	return &withTracing{inner, client, kind}
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface
	logger logr.Logger
}

func (c *withLogging) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1.Eviction) error {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Evict")
	ret0 := c.inner.Evict(arg0, arg1)
	if err := multierr.Combine(ret0); err != nil {
		logger.Error(err, "Evict failed", "duration", time.Since(start))
	} else {
		logger.Info("Evict done", "duration", time.Since(start))
	}
	return ret0
}

type withMetrics struct {
	inner    k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface
	recorder metrics.Recorder
}

func (c *withMetrics) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1.Eviction) error {
	defer c.recorder.Record("evict")
	return c.inner.Evict(arg0, arg1)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface
	client string
	kind   string
}

func (c *withTracing) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1.Eviction) error {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Evict"),
		tracing.KubeClientGroupKey.String(c.client),
		tracing.KubeClientKindKey.String(c.kind),
		tracing.KubeClientOperationKey.String("Evict"),
	)
	defer span.End()
	arg0 = ctx
	ret0 := c.inner.Evict(arg0, arg1)
	tracing.SetSpanStatus(span, ret0)
	return ret0
}
