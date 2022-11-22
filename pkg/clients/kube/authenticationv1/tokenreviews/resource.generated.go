package resource

import (
	context "context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	k8s_io_api_authentication_v1 "k8s.io/api/authentication/v1"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_client_go_kubernetes_typed_authentication_v1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface, client, kind string) k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface {
	return &withTracing{inner, client, kind}
}

type withMetrics struct {
	inner    k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface
	recorder metrics.Recorder
}

func (c *withMetrics) Create(arg0 context.Context, arg1 *k8s_io_api_authentication_v1.TokenReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authentication_v1.TokenReview, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface
	client string
	kind   string
}

func (c *withTracing) Create(arg0 context.Context, arg1 *k8s_io_api_authentication_v1.TokenReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authentication_v1.TokenReview, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Create"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.Create(arg0, arg1, arg2)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
