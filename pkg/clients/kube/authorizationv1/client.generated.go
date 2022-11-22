package client

import (
	"github.com/go-logr/logr"
	localsubjectaccessreviews "github.com/kyverno/kyverno/pkg/clients/kube/authorizationv1/localsubjectaccessreviews"
	selfsubjectaccessreviews "github.com/kyverno/kyverno/pkg/clients/kube/authorizationv1/selfsubjectaccessreviews"
	selfsubjectrulesreviews "github.com/kyverno/kyverno/pkg/clients/kube/authorizationv1/selfsubjectrulesreviews"
	subjectaccessreviews "github.com/kyverno/kyverno/pkg/clients/kube/authorizationv1/subjectaccessreviews"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_authorization_v1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface, client string) k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) LocalSubjectAccessReviews(namespace string) k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "LocalSubjectAccessReview", c.clientType)
	return localsubjectaccessreviews.WithMetrics(c.inner.LocalSubjectAccessReviews(namespace), recorder)
}
func (c *withMetrics) SelfSubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "SelfSubjectAccessReview", c.clientType)
	return selfsubjectaccessreviews.WithMetrics(c.inner.SelfSubjectAccessReviews(), recorder)
}
func (c *withMetrics) SelfSubjectRulesReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "SelfSubjectRulesReview", c.clientType)
	return selfsubjectrulesreviews.WithMetrics(c.inner.SelfSubjectRulesReviews(), recorder)
}
func (c *withMetrics) SubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "SubjectAccessReview", c.clientType)
	return subjectaccessreviews.WithMetrics(c.inner.SubjectAccessReviews(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) LocalSubjectAccessReviews(namespace string) k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface {
	return localsubjectaccessreviews.WithTracing(c.inner.LocalSubjectAccessReviews(namespace), c.client, "LocalSubjectAccessReview")
}
func (c *withTracing) SelfSubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface {
	return selfsubjectaccessreviews.WithTracing(c.inner.SelfSubjectAccessReviews(), c.client, "SelfSubjectAccessReview")
}
func (c *withTracing) SelfSubjectRulesReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface {
	return selfsubjectrulesreviews.WithTracing(c.inner.SelfSubjectRulesReviews(), c.client, "SelfSubjectRulesReview")
}
func (c *withTracing) SubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface {
	return subjectaccessreviews.WithTracing(c.inner.SubjectAccessReviews(), c.client, "SubjectAccessReview")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_authorization_v1.AuthorizationV1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) LocalSubjectAccessReviews(namespace string) k8s_io_client_go_kubernetes_typed_authorization_v1.LocalSubjectAccessReviewInterface {
	return localsubjectaccessreviews.WithLogging(c.inner.LocalSubjectAccessReviews(namespace), c.logger.WithValues("resource", "LocalSubjectAccessReviews").WithValues("namespace", namespace))
}
func (c *withLogging) SelfSubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectAccessReviewInterface {
	return selfsubjectaccessreviews.WithLogging(c.inner.SelfSubjectAccessReviews(), c.logger.WithValues("resource", "SelfSubjectAccessReviews"))
}
func (c *withLogging) SelfSubjectRulesReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SelfSubjectRulesReviewInterface {
	return selfsubjectrulesreviews.WithLogging(c.inner.SelfSubjectRulesReviews(), c.logger.WithValues("resource", "SelfSubjectRulesReviews"))
}
func (c *withLogging) SubjectAccessReviews() k8s_io_client_go_kubernetes_typed_authorization_v1.SubjectAccessReviewInterface {
	return subjectaccessreviews.WithLogging(c.inner.SubjectAccessReviews(), c.logger.WithValues("resource", "SubjectAccessReviews"))
}
