package client

import (
	"github.com/go-logr/logr"
	selfsubjectreviews "github.com/kyverno/kyverno/pkg/clients/kube/authenticationv1/selfsubjectreviews"
	tokenreviews "github.com/kyverno/kyverno/pkg/clients/kube/authenticationv1/tokenreviews"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_authentication_v1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface, client string) k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) SelfSubjectReviews() k8s_io_client_go_kubernetes_typed_authentication_v1.SelfSubjectReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "SelfSubjectReview", c.clientType)
	return selfsubjectreviews.WithMetrics(c.inner.SelfSubjectReviews(), recorder)
}
func (c *withMetrics) TokenReviews() k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "TokenReview", c.clientType)
	return tokenreviews.WithMetrics(c.inner.TokenReviews(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) SelfSubjectReviews() k8s_io_client_go_kubernetes_typed_authentication_v1.SelfSubjectReviewInterface {
	return selfsubjectreviews.WithTracing(c.inner.SelfSubjectReviews(), c.client, "SelfSubjectReview")
}
func (c *withTracing) TokenReviews() k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface {
	return tokenreviews.WithTracing(c.inner.TokenReviews(), c.client, "TokenReview")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_authentication_v1.AuthenticationV1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) SelfSubjectReviews() k8s_io_client_go_kubernetes_typed_authentication_v1.SelfSubjectReviewInterface {
	return selfsubjectreviews.WithLogging(c.inner.SelfSubjectReviews(), c.logger.WithValues("resource", "SelfSubjectReviews"))
}
func (c *withLogging) TokenReviews() k8s_io_client_go_kubernetes_typed_authentication_v1.TokenReviewInterface {
	return tokenreviews.WithLogging(c.inner.TokenReviews(), c.logger.WithValues("resource", "TokenReviews"))
}
