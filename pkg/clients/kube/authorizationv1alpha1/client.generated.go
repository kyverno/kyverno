package client

import (
	"github.com/go-logr/logr"
	authorizationconditionsreviews "github.com/kyverno/kyverno/pkg/clients/kube/authorizationv1alpha1/authorizationconditionsreviews"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_authorization_v1alpha1 "k8s.io/client-go/kubernetes/typed/authorization/v1alpha1"
	k8sclientgorest "k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationV1alpha1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) AuthorizationConditionsReviews() k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationConditionsReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "AuthorizationConditionsReview", c.clientType)
	return authorizationconditionsreviews.WithMetrics(c.inner.AuthorizationConditionsReviews(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) AuthorizationConditionsReviews() k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationConditionsReviewInterface {
	return authorizationconditionsreviews.WithTracing(c.inner.AuthorizationConditionsReviews(), c.client, "AuthorizationConditionsReview")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) AuthorizationConditionsReviews() k8s_io_client_go_kubernetes_typed_authorization_v1alpha1.AuthorizationConditionsReviewInterface {
	return authorizationconditionsreviews.WithLogging(c.inner.AuthorizationConditionsReviews(), c.logger.WithValues("resource", "AuthorizationConditionsReviews"))
}
