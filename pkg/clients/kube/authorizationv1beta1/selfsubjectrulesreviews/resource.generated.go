package resource

import (
	context "context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	k8s_io_api_authorization_v1beta1 "k8s.io/api/authorization/v1beta1"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_client_go_kubernetes_typed_authorization_v1beta1 "k8s.io/client-go/kubernetes/typed/authorization/v1beta1"
)

func WithLogging(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface {
	return &withLogging{inner, logger}
}

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface, client, kind string) k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface {
	return &withTracing{inner, client, kind}
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface
	logger logr.Logger
}

func (c *withLogging) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.SelfSubjectRulesReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1beta1.SelfSubjectRulesReview, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Create")
	ret0, ret1 := c.inner.Create(arg0, arg1, arg2)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "Create failed", "duration", time.Since(start))
	} else {
		logger.Info("Create done", "duration", time.Since(start))
	}
	return ret0, ret1
}

type withMetrics struct {
	inner    k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface
	recorder metrics.Recorder
}

func (c *withMetrics) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.SelfSubjectRulesReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1beta1.SelfSubjectRulesReview, error) {
	defer c.recorder.RecordWithContext(arg0, "create")
	return c.inner.Create(arg0, arg1, arg2)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_authorization_v1beta1.SelfSubjectRulesReviewInterface
	client string
	kind   string
}

func (c *withTracing) Create(arg0 context.Context, arg1 *k8s_io_api_authorization_v1beta1.SelfSubjectRulesReview, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_authorization_v1beta1.SelfSubjectRulesReview, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Create"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("Create"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.Create(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
