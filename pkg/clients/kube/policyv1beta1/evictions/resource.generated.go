package resource

import (
	context "context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	k8s_io_api_policy_v1beta1 "k8s.io/api/policy/v1beta1"
	k8s_io_client_go_kubernetes_typed_policy_v1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface, client, kind string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface {
	return &withTracing{inner, client, kind}
}

type withMetrics struct {
	inner    k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface
	recorder metrics.Recorder
}

func (c *withMetrics) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.Eviction) error {
	defer c.recorder.Record("evict")
	return c.inner.Evict(arg0, arg1)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface
	client string
	kind   string
}

func (c *withTracing) Evict(arg0 context.Context, arg1 *k8s_io_api_policy_v1beta1.Eviction) error {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Evict"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "Evict"),
	)
	defer span.End()
	arg0 = ctx
	ret0 := c.inner.Evict(arg0, arg1)
	if ret0 != nil {
		span.RecordError(ret0)
		span.SetStatus(codes.Error, ret0.Error())
	}
	return ret0
}
